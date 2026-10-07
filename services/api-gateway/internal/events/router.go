// Package events — доставка событий из Kafka в WebSocket: разбор конверта, выбор получателей, отправка через Hub.
package events

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/hub"
)

const (
	TopicAuthEvents         = "auth-events"
	TopicUserEvents         = "user-events"
	TopicConversationEvents = "conversation-events"
	TopicMessageEvents      = "message-events"

	TypeSessionRevoked = "session.revoked"
	TypeAccountDeleted = "user.account_deleted"

	TypeUserBlocked   = "user.blocked"
	TypeUserUnblocked = "user.unblocked"

	TypeConversationCreated = "conversation.created"
	TypeConversationUpdated = "conversation.updated"
	TypeConversationDeleted = "conversation.deleted"
	TypeMemberAdded         = "conversation.member.added"
	TypeMemberRemoved       = "conversation.member.removed"
	TypeJoinRequestCreated  = "conversation.join_request.created"
	TypeJoinRequestApproved = "conversation.join_request.approved"

	TypeMessageCreated = "message.created"
	TypeMessageUpdated = "message.updated"
	TypeMessageDeleted = "message.deleted"
	TypeMessageRead    = "message.read"
)

// Topics — топики, которые читает Gateway.
var Topics = []string{TopicAuthEvents, TopicUserEvents, TopicConversationEvents, TopicMessageEvents}

// Поля маршрутизации и шифртекст клиенту не отправляются.
var hiddenFields = []string{"member_ids", "admin_ids", "author_ids", "content_enc"}

// ErrMalformed — событие нельзя разобрать; повтор его не исправит.
var ErrMalformed = errors.New("malformed event")

// Sink — часть Hub, нужная маршрутизации.
type Sink interface {
	SendToUsers(userIDs []string, msg []byte)
	CloseSID(sid string, final []byte, code int, reason string)
	CloseUser(userID string, final []byte, code int, reason string)
}

// ProfileCache — сброс кеша профиля при удалении аккаунта.
type ProfileCache interface {
	Deleted(ctx context.Context, userID string) error
}

type envelope struct {
	EventID    string          `json:"event_id"`
	EventType  string          `json:"event_type"`
	OccurredAt time.Time       `json:"occurred_at"`
	Payload    json.RawMessage `json:"payload"`
}

// recipients — поля payload, по которым выбираются получатели.
type recipients struct {
	MemberIDs []string `json:"member_ids"`
	AdminIDs  []string `json:"admin_ids"`
	AuthorIDs []string `json:"author_ids"`
	ReaderID  string   `json:"reader_id"`
	BlockerID string   `json:"blocker_id"`
	BlockedID string   `json:"blocked_id"`
	SessionID string   `json:"session_id"`
	UserID    string   `json:"user_id"`
}

// delivery — формат кадра для клиента.
type delivery struct {
	Type       string          `json:"type"`
	EventID    string          `json:"event_id"`
	OccurredAt time.Time       `json:"occurred_at"`
	Data       json.RawMessage `json:"data"`
}

type Options struct {
	// MaxAge — события старше доставляются клиентам не будут: после простоя клиенты синхронизируются через REST.
	MaxAge time.Duration
	// DedupTTL / DedupSize — сколько и как долго помнить обработанные event_id.
	DedupTTL  time.Duration
	DedupSize int
}

func DefaultOptions() Options {
	return Options{MaxAge: time.Minute, DedupTTL: time.Hour, DedupSize: 100_000}
}

type Router struct {
	sink     Sink
	profiles ProfileCache
	log      *zap.Logger
	maxAge   time.Duration
	seen     *dedup
	now      func() time.Time
}

func NewRouter(sink Sink, profiles ProfileCache, opts Options, log *zap.Logger) *Router {
	return &Router{
		sink: sink, profiles: profiles, log: log, maxAge: opts.MaxAge,
		seen: newDedup(opts.DedupTTL, opts.DedupSize), now: time.Now,
	}
}

// Handle обрабатывает одно событие. Ошибка, кроме ErrMalformed, означает временный сбой: событие стоит повторить.
// Неизвестные типы пропускаются без ошибки.
func (r *Router) Handle(ctx context.Context, value []byte) error {
	var ev envelope
	if err := json.Unmarshal(value, &ev); err != nil || ev.EventID == "" || ev.EventType == "" || len(ev.Payload) == 0 {
		return ErrMalformed
	}
	now := r.now()
	if r.seen.has(ev.EventID, now) {
		return nil
	}
	var to recipients
	if err := json.Unmarshal(ev.Payload, &to); err != nil {
		return ErrMalformed
	}

	if err := r.route(ctx, ev, to, now); err != nil {
		return err
	}
	r.seen.add(ev.EventID, now)
	return nil
}

func (r *Router) route(ctx context.Context, ev envelope, to recipients, now time.Time) error {
	log := r.log.With(zap.String("event_id", ev.EventID), zap.String("event_type", ev.EventType))

	// События безопасности обрабатываются независимо от возраста: отозванная сессия не должна оставаться на связи.
	switch ev.EventType {
	case TypeSessionRevoked:
		if to.SessionID == "" {
			return ErrMalformed
		}
		final, err := r.frame(ev)
		if err != nil {
			return err
		}
		r.sink.CloseSID(to.SessionID, final, hub.CloseSessionRevoked, "session revoked")
		log.Debug("ws session closed", zap.String("sid", to.SessionID))
		return nil
	case TypeAccountDeleted:
		if to.UserID == "" {
			return ErrMalformed
		}
		if err := r.profiles.Deleted(ctx, to.UserID); err != nil {
			return err
		}
		final, err := r.frame(ev)
		if err != nil {
			return err
		}
		r.sink.CloseUser(to.UserID, final, hub.CloseSessionRevoked, "account deleted")
		log.Debug("ws user closed", zap.String("user_id", to.UserID))
		return nil
	}

	var users []string
	switch ev.EventType {
	case TypeMessageCreated, TypeMessageUpdated, TypeMessageDeleted,
		TypeConversationCreated, TypeConversationUpdated, TypeConversationDeleted,
		TypeMemberAdded, TypeMemberRemoved, TypeJoinRequestApproved:
		users = to.MemberIDs
	case TypeMessageRead:
		// Читатель получает событие во все свои вкладки: счётчик непрочитанных синхронизируется между ними.
		users = append(append([]string{}, to.AuthorIDs...), to.ReaderID)
	case TypeJoinRequestCreated:
		users = to.AdminIDs
	case TypeUserBlocked, TypeUserUnblocked:
		users = []string{to.BlockerID, to.BlockedID}
	default:
		log.Debug("event type not delivered")
		return nil
	}

	if ev.OccurredAt.IsZero() {
		return ErrMalformed
	}
	if age := now.Sub(ev.OccurredAt); age > r.maxAge {
		log.Debug("stale event skipped", zap.Duration("age", age))
		return nil
	}
	users = unique(users)
	if len(users) == 0 {
		return nil
	}
	msg, err := r.frame(ev)
	if err != nil {
		return err
	}
	r.sink.SendToUsers(users, msg)
	log.Debug("event delivered", zap.Int("recipients", len(users)))
	return nil
}

// frame собирает кадр для клиента, вырезая поля маршрутизации из payload.
func (r *Router) frame(ev envelope) ([]byte, error) {
	var data map[string]json.RawMessage
	if err := json.Unmarshal(ev.Payload, &data); err != nil {
		return nil, ErrMalformed
	}
	for _, f := range hiddenFields {
		delete(data, f)
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, ErrMalformed
	}
	msg, err := json.Marshal(delivery{Type: ev.EventType, EventID: ev.EventID, OccurredAt: ev.OccurredAt, Data: payload})
	if err != nil {
		return nil, ErrMalformed
	}
	return msg, nil
}

func unique(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	res := ids[:0:0]
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		res = append(res, id)
	}
	return res
}
