package presence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	conversationv1 "github.com/maltira/chavo-project-backend/proto/gen/go/conversation/v1"
	userv1 "github.com/maltira/chavo-project-backend/proto/gen/go/user/v1"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/hub"
)

type nopRegistry struct{}

func (nopRegistry) Add(context.Context, string, string) error    { return nil }
func (nopRegistry) Remove(context.Context, string, string) error { return nil }

type fakeUsers struct {
	userv1.UserInternalServiceClient
	mu       sync.Mutex
	hidden   map[string]bool
	lastSeen map[string]time.Time
	err      error
}

func (f *fakeUsers) PresenceVisible(_ context.Context, req *userv1.PresenceVisibleRequest, _ ...grpc.CallOption) (*userv1.PresenceVisibleResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	resp := &userv1.PresenceVisibleResponse{Visible: map[string]bool{}, LastSeenAt: map[string]*timestamppb.Timestamp{}}
	for _, id := range req.GetUserIds() {
		resp.Visible[id] = !f.hidden[id]
		if at, ok := f.lastSeen[id]; ok && req.GetWithLastSeen() && !f.hidden[id] {
			resp.LastSeenAt[id] = timestamppb.New(at)
		}
	}
	return resp, nil
}

type fakePeers struct {
	conversationv1.ConversationInternalServiceClient
	peers map[string][]string
}

func (f *fakePeers) ListDirectPeers(_ context.Context, req *conversationv1.ListDirectPeersRequest, _ ...grpc.CallOption) (*conversationv1.ListDirectPeersResponse, error) {
	return &conversationv1.ListDirectPeersResponse{UserIds: f.peers[req.GetUserId()]}, nil
}

type fakePublisher struct {
	mu     sync.Mutex
	events []Event
	closed bool
}

func (p *fakePublisher) Publish(_ context.Context, key string, value []byte) error {
	var ev Event
	if err := json.Unmarshal(value, &ev); err != nil {
		return err
	}
	if key != ev.Payload.UserID {
		return errors.New("key must be user_id")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, ev)
	return nil
}

func (p *fakePublisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	return nil
}

func (p *fakePublisher) types() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	res := make([]string, len(p.events))
	for i, ev := range p.events {
		res[i] = ev.EventType + ":" + ev.Payload.UserID
	}
	return res
}

func (p *fakePublisher) last() Event {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.events[len(p.events)-1]
}

type env struct {
	t     *testing.T
	hub   *hub.Hub
	svc   *Service
	users *fakeUsers
	peers *fakePeers
	pub   *fakePublisher
}

const delay = 80 * time.Millisecond

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{
		t:     t,
		hub:   hub.New(nopRegistry{}, zap.NewNop()),
		users: &fakeUsers{hidden: map[string]bool{}, lastSeen: map[string]time.Time{}},
		peers: &fakePeers{peers: map[string][]string{}},
		pub:   &fakePublisher{},
	}
	opts := DefaultOptions()
	opts.OfflineDelay = delay
	opts.MaxPerMessage, opts.MaxSubscriptions = 3, 4
	e.svc = New(e.hub, e.users, e.peers, e.pub, opts, zap.NewNop())
	t.Cleanup(func() { _ = e.svc.Shutdown(context.Background()) })
	return e
}

// connect регистрирует соединение так же, как ws.serve.
func (e *env) connect(user string) *hub.Conn {
	e.t.Helper()
	c := hub.NewConn(user+":"+uuid.NewString()[:8], user, "sid-"+user, 64)
	first, err := e.hub.Register(context.Background(), c)
	if err != nil {
		e.t.Fatal(err)
	}
	e.svc.Connected(context.Background(), c, first)
	return c
}

// settle ждёт, пока очередь рассылки выполнит всё, что в ней уже есть.
func (e *env) settle() {
	done := make(chan struct{})
	e.svc.jobs.push(func() { close(done) })
	<-done
}

func (e *env) disconnect(c *hub.Conn) {
	c.Close(hub.CloseGoingAway, "")
	e.svc.Disconnected(c, e.hub.Unregister(c))
}

type frame struct {
	Type string `json:"type"`
	Data struct {
		UserID     string          `json:"user_id"`
		LastSeenAt *time.Time      `json:"last_seen_at"`
		Users      []snapshotEntry `json:"users"`
	} `json:"data"`
}

func (e *env) next(c *hub.Conn) frame {
	e.t.Helper()
	select {
	case msg := <-c.Send():
		var f frame
		if err := json.Unmarshal(msg, &f); err != nil {
			e.t.Fatal(err)
		}
		return f
	case <-time.After(2 * time.Second):
		e.t.Fatal("no frame")
		return frame{}
	}
}

func (e *env) none(c *hub.Conn, wait time.Duration) {
	e.t.Helper()
	select {
	case msg := <-c.Send():
		e.t.Fatalf("unexpected frame %s", msg)
	case <-time.After(wait):
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (e *env) watcher(target string) *hub.Conn {
	e.t.Helper()
	w := e.connect(uuid.NewString())
	if err := e.svc.Subscribe(context.Background(), w, []string{target}); err != nil {
		e.t.Fatal(err)
	}
	if f := e.next(w); f.Type != TypeSnapshot {
		e.t.Fatalf("expected snapshot, got %s", f.Type)
	}
	return w
}

func TestOnlineOfflineWithDelay(t *testing.T) {
	e := newEnv(t)
	u := uuid.NewString()
	w := e.watcher(u)

	tab1, tab2 := e.connect(u), e.connect(u)
	if f := e.next(w); f.Type != TypeUserOnline || f.Data.UserID != u || f.Data.LastSeenAt != nil {
		t.Fatalf("got %+v", f)
	}

	e.disconnect(tab1)
	before := time.Now()
	e.disconnect(tab2)
	after := time.Now()
	// До истечения задержки пользователь остаётся online.
	e.none(w, delay/2)

	f := e.next(w)
	if f.Type != TypeUserOffline || f.Data.LastSeenAt == nil {
		t.Fatalf("got %+v", f)
	}
	// last_seen_at — момент закрытия последней вкладки, а не истечения задержки.
	if f.Data.LastSeenAt.Before(before.Add(-time.Millisecond)) || f.Data.LastSeenAt.After(after) {
		t.Fatalf("last_seen_at %v not in [%v, %v]", f.Data.LastSeenAt, before, after)
	}
	waitFor(t, "kafka events", func() bool { return len(e.pub.types()) >= 3 })
	got := strings.Join(e.pub.types(), ",")
	if !strings.HasSuffix(got, TypeUserOnline+":"+u+","+TypeUserOffline+":"+u) {
		t.Fatalf("published %s", got)
	}
	if ev := e.pub.last(); !ev.Payload.Visible || !ev.Payload.At.Equal(*f.Data.LastSeenAt) || ev.EventID == "" {
		t.Fatalf("offline event %+v", ev)
	}
}

func TestReconnectWithinDelayIsInvisible(t *testing.T) {
	e := newEnv(t)
	u := uuid.NewString()
	w := e.watcher(u)
	e.disconnect(e.connect(u))
	if f := e.next(w); f.Type != TypeUserOnline {
		t.Fatalf("got %+v", f)
	}
	time.Sleep(delay / 2)
	e.connect(u)
	e.none(w, 2*delay)
	for _, typ := range e.pub.types() {
		if strings.HasPrefix(typ, TypeUserOffline+":"+u) {
			t.Fatalf("offline published on reconnect: %v", e.pub.types())
		}
	}
}

func TestHiddenUser(t *testing.T) {
	e := newEnv(t)
	u := uuid.NewString()
	e.users.hidden[u] = true
	e.users.lastSeen[u] = time.Now()
	w := e.watcher(u)
	e.connect(u)
	e.none(w, delay)

	waitFor(t, "kafka event", func() bool { return len(e.pub.types()) >= 2 })
	if ev := e.pub.last(); ev.Payload.UserID != u || ev.Payload.Visible {
		t.Fatalf("event %+v", ev)
	}

	// В снимке скрытый пользователь всегда offline и без last_seen_at.
	w2 := e.connect(uuid.NewString())
	if err := e.svc.Subscribe(context.Background(), w2, []string{u}); err != nil {
		t.Fatal(err)
	}
	f := e.next(w2)
	if len(f.Data.Users) != 1 || f.Data.Users[0].Online || f.Data.Users[0].LastSeenAt != nil {
		t.Fatalf("snapshot %+v", f.Data.Users)
	}
}

func TestVisibilityErrorHidesUser(t *testing.T) {
	e := newEnv(t)
	u := uuid.NewString()
	w := e.watcher(u)
	e.users.mu.Lock()
	e.users.err = errors.New("user-service down")
	e.users.mu.Unlock()
	e.connect(u)
	e.none(w, delay)
}

func TestSubscribeSnapshotAndValidation(t *testing.T) {
	e := newEnv(t)
	on, off, seen := uuid.NewString(), uuid.NewString(), time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	e.users.lastSeen[off] = seen
	e.connect(on)
	e.settle()

	c := e.connect(uuid.NewString())
	ctx := context.Background()
	// Повторы и разный регистр схлопываются.
	if err := e.svc.Subscribe(ctx, c, []string{on, strings.ToUpper(off), off}); err != nil {
		t.Fatal(err)
	}
	f := e.next(c)
	if f.Type != TypeSnapshot || len(f.Data.Users) != 2 {
		t.Fatalf("snapshot %+v", f)
	}
	if u := f.Data.Users[0]; u.UserID != on || !u.Online {
		t.Fatalf("online entry %+v", u)
	}
	if u := f.Data.Users[1]; u.UserID != off || u.Online || u.LastSeenAt == nil || !u.LastSeenAt.Equal(seen) {
		t.Fatalf("offline entry %+v", u)
	}

	for _, tc := range []struct {
		ids  []string
		want error
	}{
		{nil, ErrInvalidIDs},
		{[]string{"not-a-uuid"}, ErrInvalidIDs},
		{[]string{uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()}, ErrTooMany},
		// 2 уже есть, лимит соединения — 4.
		{[]string{uuid.NewString(), uuid.NewString(), uuid.NewString()}, hub.ErrWatchLimit},
	} {
		if err := e.svc.Subscribe(ctx, c, tc.ids); !errors.Is(err, tc.want) {
			t.Fatalf("%v: got %v, want %v", tc.ids, err, tc.want)
		}
	}

	// После отписки переходы не приходят.
	if err := e.svc.Unsubscribe(c, []string{off}); err != nil {
		t.Fatal(err)
	}
	e.connect(off)
	e.none(c, delay)
}

func TestDirectPeersWatchedOnConnect(t *testing.T) {
	e := newEnv(t)
	me, peer := uuid.NewString(), uuid.NewString()
	e.peers.peers[me] = []string{peer}

	c := e.connect(me)
	f := e.next(c)
	if f.Type != TypeSnapshot || len(f.Data.Users) != 1 || f.Data.Users[0].UserID != peer || f.Data.Users[0].Online {
		t.Fatalf("snapshot %+v", f)
	}

	e.connect(peer)
	if f := e.next(c); f.Type != TypeUserOnline || f.Data.UserID != peer {
		t.Fatalf("got %+v", f)
	}
	// Собеседник по direct не снимается явной отпиской.
	if err := e.svc.Unsubscribe(c, []string{peer}); err != nil {
		t.Fatal(err)
	}
	e.disconnect(e.connect(peer)) // вторая вкладка собеседника: перехода нет
	e.none(c, delay/2)
}

func TestDirectCreated(t *testing.T) {
	e := newEnv(t)
	a, b := uuid.NewString(), uuid.NewString()
	ca, cb := e.connect(a), e.connect(b)
	e.settle()

	e.svc.DirectCreated(context.Background(), a, b)
	for _, tc := range []struct {
		c    *hub.Conn
		peer string
	}{{ca, b}, {cb, a}} {
		f := e.next(tc.c)
		if f.Type != TypeSnapshot || len(f.Data.Users) != 1 || f.Data.Users[0].UserID != tc.peer || !f.Data.Users[0].Online {
			t.Fatalf("snapshot %+v", f)
		}
	}

	e.disconnect(cb)
	if f := e.next(ca); f.Type != TypeUserOffline || f.Data.UserID != b {
		t.Fatalf("got %+v", f)
	}
}

func TestShutdownFlushesOffline(t *testing.T) {
	e := newEnv(t)
	gone, stay := uuid.NewString(), uuid.NewString()
	e.disconnect(e.connect(gone))
	at := time.Now()
	e.connect(stay)

	if err := e.svc.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Задержка не дожидается: offline объявлен обоим сразу.
	got := e.pub.types()
	offline := map[string]time.Time{}
	e.pub.mu.Lock()
	for _, ev := range e.pub.events {
		if ev.EventType == TypeUserOffline {
			offline[ev.Payload.UserID] = ev.Payload.At
		}
	}
	closed := e.pub.closed
	e.pub.mu.Unlock()
	if len(offline) != 2 || !closed {
		t.Fatalf("published %v, closed %v", got, closed)
	}
	if offline[gone].After(at) {
		t.Fatal("pending offline must keep the disconnect time")
	}

	// После остановки новые переходы не публикуются.
	e.connect(uuid.NewString())
	time.Sleep(delay / 2)
	if len(e.pub.types()) != len(got) {
		t.Fatal("event published after shutdown")
	}
}
