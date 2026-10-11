package events

import (
	"time"

	"github.com/google/uuid"
)

const (
	TopicUserEvents = "user-events"

	TypeUserBlocked   = "user.blocked"
	TypeUserUnblocked = "user.unblocked"

	// Публикует Gateway
	TopicPresenceEvents = "presence-events"
	TypeUserOnline      = "user.online"
	TypeUserOffline     = "user.offline"
)

type Event[T any] struct {
	EventID    string    `json:"event_id"`
	EventType  string    `json:"event_type"`
	OccurredAt time.Time `json:"occurred_at"`
	Payload    T         `json:"payload"`
}

func NewEvent[T any](eventType string, payload T) Event[T] {
	return Event[T]{
		EventID:    uuid.NewString(),
		EventType:  eventType,
		OccurredAt: time.Now().UTC(),
		Payload:    payload,
	}
}

type BlockPayload struct {
	BlockerID uuid.UUID `json:"blocker_id"`
	BlockedID uuid.UUID `json:"blocked_id"`
}

// PresencePayload — переход online/offline; At — момент перехода по часам Gateway
type PresencePayload struct {
	UserID  uuid.UUID `json:"user_id"`
	At      time.Time `json:"at"`
	Visible bool      `json:"visible"`
}
