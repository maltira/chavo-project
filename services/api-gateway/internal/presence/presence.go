// Package presence считает переходы online/offline по соединениям Hub, публикует их в presence-events
// и доставляет наблюдателям: собеседникам по direct и тем, кто подписался явно.
//
// Gateway один, поэтому его состояние — источник истины об онлайне. Видимость (show_online_status)
// и last_seen_at хранит user-service.
package presence

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	conversationv1 "github.com/maltira/chavo-project-backend/proto/gen/go/conversation/v1"
	userv1 "github.com/maltira/chavo-project-backend/proto/gen/go/user/v1"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/hub"
)

const (
	TypeUserOnline  = "user.online"
	TypeUserOffline = "user.offline"
	TypeSnapshot    = "presence.snapshot"

	// visibleBatch — лимит PresenceVisible в user-service.
	visibleBatch = 1000
)

var (
	ErrInvalidIDs = errors.New("invalid user ids")
	ErrTooMany    = errors.New("too many user ids")
)

type Options struct {
	// OfflineDelay — offline объявляется, только если пользователь не вернулся за это время (reconnect, перезагрузка вкладки).
	OfflineDelay time.Duration
	// MaxPerMessage — user_ids в одном presence.subscribe/unsubscribe; MaxSubscriptions — явных подписок на соединение.
	MaxPerMessage    int
	MaxSubscriptions int
}

func DefaultOptions() Options {
	return Options{OfflineDelay: 10 * time.Second, MaxPerMessage: 200, MaxSubscriptions: 1000}
}

// Publisher отправляет переходы в presence-events (key = user_id).
type Publisher interface {
	Publish(ctx context.Context, key string, value []byte) error
	Close() error
}

// Event — конверт presence-events, тот же формат, что у остальных сервисов.
type Event struct {
	EventID    string    `json:"event_id"`
	EventType  string    `json:"event_type"`
	OccurredAt time.Time `json:"occurred_at"`
	Payload    Payload   `json:"payload"`
}

type Payload struct {
	UserID  string    `json:"user_id"`
	At      time.Time `json:"at"`
	Visible bool      `json:"visible"`
}

type Service struct {
	hub   *hub.Hub
	users userv1.UserInternalServiceClient
	peers conversationv1.ConversationInternalServiceClient
	pub   Publisher
	opts  Options
	log   *zap.Logger
	now   func() time.Time

	mu sync.Mutex
	// online — объявленное состояние (с учётом задержки offline); pending — ждущие объявления offline.
	online  map[string]bool
	pending map[string]*pendingOffline
	closed  bool

	// Все рассылки идут через одну очередь: переходы одного пользователя и снимки не обгоняют друг друга.
	jobs       *queue
	workerDone chan struct{}
}

type pendingOffline struct {
	timer *time.Timer
	at    time.Time
}

func New(h *hub.Hub, users userv1.UserInternalServiceClient, peers conversationv1.ConversationInternalServiceClient,
	pub Publisher, opts Options, log *zap.Logger) *Service {
	s := &Service{
		hub: h, users: users, peers: peers, pub: pub, opts: opts, log: log, now: time.Now,
		online: map[string]bool{}, pending: map[string]*pendingOffline{},
		jobs: newQueue(), workerDone: make(chan struct{}),
	}
	go func() {
		defer close(s.workerDone)
		s.jobs.run()
	}()
	return s
}

// ── переходы ─────────────────────────────────────────

// Connected вызывается после регистрации соединения в Hub; first — первое соединение пользователя.
// Собеседники по direct загружаются в фоне и приходят клиенту снимком.
func (s *Service) Connected(ctx context.Context, c *hub.Conn, first bool) {
	if first {
		s.markOnline(c.UserID)
	}
	go s.attachDirect(context.WithoutCancel(ctx), c)
}

// Disconnected вызывается после снятия соединения с учёта; last — соединений пользователя не осталось.
func (s *Service) Disconnected(c *hub.Conn, last bool) {
	if !last {
		return
	}
	u := c.UserID
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || !s.online[u] || s.pending[u] != nil {
		return
	}
	p := &pendingOffline{at: s.now()}
	p.timer = time.AfterFunc(s.opts.OfflineDelay, func() { s.expire(u, p) })
	s.pending[u] = p
}

func (s *Service) markOnline(u string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if p := s.pending[u]; p != nil {
		// Вернулся до истечения задержки: для остальных он и не уходил.
		p.timer.Stop()
		delete(s.pending, u)
	}
	if s.online[u] {
		return
	}
	s.online[u] = true
	at := s.now()
	s.jobs.push(func() { s.emit(u, true, at) })
}

func (s *Service) expire(u string, p *pendingOffline) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending[u] != p {
		return
	}
	delete(s.pending, u)
	// Новое соединение уже в Hub, но его Connected ещё не дошёл: пользователь остаётся online.
	if s.hub.Online(u) {
		return
	}
	delete(s.online, u)
	s.jobs.push(func() { s.emit(u, false, p.at) })
}

// emit публикует переход в Kafka и, если пользователь показывает онлайн, рассылает его наблюдателям.
// Для offline at — момент закрытия последнего соединения (last_seen_at), а не истечения задержки.
func (s *Service) emit(u string, online bool, at time.Time) {
	ctx := context.Background()
	visible := false
	if resp, err := s.users.PresenceVisible(ctx, &userv1.PresenceVisibleRequest{UserIds: []string{u}}); err != nil {
		// Без ответа user-service онлайн не показывается: скрытый пользователь не должен «засветиться».
		s.log.Warn("presence visibility check failed", zap.String("user_id", u), zap.Error(err))
	} else {
		visible = resp.GetVisible()[u]
	}

	typ := TypeUserOffline
	if online {
		typ = TypeUserOnline
	}
	ev := Event{EventID: uuid.NewString(), EventType: typ, OccurredAt: s.now().UTC(), Payload: Payload{UserID: u, At: at.UTC(), Visible: visible}}
	value, _ := json.Marshal(ev)
	if err := s.pub.Publish(ctx, u, value); err != nil {
		s.log.Error("presence publish failed", zap.String("user_id", u), zap.String("event_type", typ), zap.Error(err))
	}
	if visible {
		s.hub.SendToWatchers(u, transitionFrame(ev))
	}
	s.log.Debug("presence transition", zap.String("user_id", u), zap.String("event_type", typ), zap.Bool("visible", visible))
}

// ── наблюдение ───────────────────────────────────────

func (s *Service) attachDirect(ctx context.Context, c *hub.Conn) {
	resp, err := s.peers.ListDirectPeers(ctx, &conversationv1.ListDirectPeersRequest{UserId: c.UserID})
	if err != nil {
		s.log.Warn("list direct peers failed", zap.String("connection_id", c.ID), zap.Error(err))
		return
	}
	ids := resp.GetUserIds()
	if len(ids) == 0 {
		return
	}
	s.hub.WatchDirect(c, ids)
	if err := s.snapshot(ctx, ids, c.Enqueue); err != nil {
		s.log.Warn("direct presence snapshot failed", zap.String("connection_id", c.ID), zap.Error(err))
	}
}

// DirectCreated — новый direct-чат: собеседники начинают видеть присутствие друг друга без переподключения.
func (s *Service) DirectCreated(ctx context.Context, a, b string) {
	s.hub.WatchPeers(a, b)
	ctx = context.WithoutCancel(ctx)
	go func() {
		for _, p := range [2][2]string{{a, b}, {b, a}} {
			to := p[0]
			if err := s.snapshot(ctx, []string{p[1]}, func(msg []byte) bool { s.hub.SendToUsers([]string{to}, msg); return true }); err != nil {
				s.log.Warn("direct presence snapshot failed", zap.String("user_id", to), zap.Error(err))
			}
		}
	}()
}

// Subscribe — presence.subscribe: подписка и снимок текущего состояния в ответ.
func (s *Service) Subscribe(ctx context.Context, c *hub.Conn, userIDs []string) error {
	ids, err := s.normalize(userIDs)
	if err != nil {
		return err
	}
	if err := s.hub.Subscribe(c, ids, s.opts.MaxSubscriptions); err != nil {
		return err
	}
	return s.snapshot(ctx, ids, c.Enqueue)
}

// Unsubscribe — presence.unsubscribe; собеседники по direct не снимаются.
func (s *Service) Unsubscribe(c *hub.Conn, userIDs []string) error {
	ids, err := s.normalize(userIDs)
	if err != nil {
		return err
	}
	s.hub.Unsubscribe(c, ids)
	return nil
}

func (s *Service) normalize(userIDs []string) ([]string, error) {
	if len(userIDs) == 0 {
		return nil, ErrInvalidIDs
	}
	if len(userIDs) > s.opts.MaxPerMessage {
		return nil, ErrTooMany
	}
	seen := make(map[string]struct{}, len(userIDs))
	res := make([]string, 0, len(userIDs))
	for _, raw := range userIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, ErrInvalidIDs
		}
		key := id.String()
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			res = append(res, key)
		}
	}
	return res, nil
}

// snapshot запрашивает видимость и last_seen_at, а состояние online читает уже в очереди рассылки:
// переход, объявленный раньше, придёт клиенту раньше снимка, а более поздний — после него.
func (s *Service) snapshot(ctx context.Context, ids []string, deliver func([]byte) bool) error {
	visible := make(map[string]bool, len(ids))
	lastSeen := make(map[string]*timestamppb.Timestamp, len(ids))
	for start := 0; start < len(ids); start += visibleBatch {
		end := min(start+visibleBatch, len(ids))
		resp, err := s.users.PresenceVisible(ctx, &userv1.PresenceVisibleRequest{UserIds: ids[start:end], WithLastSeen: true})
		if err != nil {
			return err
		}
		for id, v := range resp.GetVisible() {
			visible[id] = v
		}
		for id, at := range resp.GetLastSeenAt() {
			lastSeen[id] = at
		}
	}

	s.jobs.push(func() {
		entries := make([]snapshotEntry, len(ids))
		s.mu.Lock()
		for i, id := range ids {
			e := snapshotEntry{UserID: id}
			if visible[id] {
				e.Online = s.online[id]
				if at := lastSeen[id]; at != nil {
					t := at.AsTime()
					e.LastSeenAt = &t
				}
			}
			entries[i] = e
		}
		s.mu.Unlock()
		deliver(snapshotFrame(entries))
	})
	return nil
}

// ── остановка ────────────────────────────────────────

// Shutdown вызывается после закрытия всех соединений Hub: объявляет offline сразу, не дожидаясь задержки
// (иначе last_seen_at этих пользователей не обновится), и дожидается отправки в Kafka.
func (s *Service) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	now := s.now()
	for u := range s.online {
		at := now
		if p := s.pending[u]; p != nil {
			p.timer.Stop()
			at = p.at
		}
		s.jobs.push(func() { s.emit(u, false, at) })
	}
	s.online, s.pending = map[string]bool{}, map[string]*pendingOffline{}
	s.mu.Unlock()

	s.jobs.close()
	select {
	case <-s.workerDone:
	case <-ctx.Done():
		return ctx.Err()
	}
	return s.pub.Close()
}

// ── кадры клиенту ────────────────────────────────────

type transitionData struct {
	UserID     string     `json:"user_id"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
}

func transitionFrame(ev Event) []byte {
	data := transitionData{UserID: ev.Payload.UserID}
	if ev.EventType == TypeUserOffline {
		at := ev.Payload.At
		data.LastSeenAt = &at
	}
	b, _ := json.Marshal(struct {
		Type       string         `json:"type"`
		EventID    string         `json:"event_id"`
		OccurredAt time.Time      `json:"occurred_at"`
		Data       transitionData `json:"data"`
	}{ev.EventType, ev.EventID, ev.OccurredAt, data})
	return b
}

type snapshotEntry struct {
	UserID     string     `json:"user_id"`
	Online     bool       `json:"online"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
}

func snapshotFrame(entries []snapshotEntry) []byte {
	type data struct {
		Users []snapshotEntry `json:"users"`
	}
	b, _ := json.Marshal(struct {
		Type string `json:"type"`
		Data data   `json:"data"`
	}{TypeSnapshot, data{entries}})
	return b
}
