package events

import (
	"time"

	"github.com/google/uuid"
)

const (
	TopicConversationEvents = "conversation-events"
	TopicMessageEvents      = "message-events"

	TypeConversationCreated = "conversation.created"
	TypeConversationUpdated = "conversation.updated"
	TypeConversationDeleted = "conversation.deleted"

	TypeMemberAdded   = "conversation.member.added"
	TypeMemberRemoved = "conversation.member.removed"

	TypeJoinRequestCreated  = "conversation.join_request.created"
	TypeJoinRequestApproved = "conversation.join_request.approved"

	TypeMessageCreated = "message.created"
	TypeMessageUpdated = "message.updated"
	TypeMessageDeleted = "message.deleted"
	TypeMessageRead    = "message.read"
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
