package events

import (
	"time"

	"github.com/google/uuid"
)

const (
	TopicUserEvents = "user-events"

	TypeUserBlocked   = "user.blocked"
	TypeUserUnblocked = "user.unblocked"
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
