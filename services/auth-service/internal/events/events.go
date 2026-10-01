package events

import (
	"time"

	"github.com/google/uuid"
)

const (
	TopicAuthEvents = "auth-events"

	TypeSessionRevoked = "session.revoked"
	TypeAccountDeleted = "user.account_deleted"
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

type SessionRevokedPayload struct {
	SessionID uuid.UUID `json:"session_id"`
	UserID    uuid.UUID `json:"user_id"`
	Reason    string    `json:"reason"`
}

type AccountDeletedPayload struct {
	UserID uuid.UUID `json:"user_id"`
}
