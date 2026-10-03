package events

import (
	"time"

	"github.com/google/uuid"
)

// MemberIDs нужны Gateway, чтобы определить получателей.
// Текст сообщений: в outbox лежит шифртекст (ContentEnc), publisher расшифровывает его перед отправкой
// в Kafka и заполняет Content. В БД plaintext не попадает.

type ConversationCreatedPayload struct {
	ConversationID   uuid.UUID   `json:"conversation_id"`
	ConversationType string      `json:"conversation_type"`
	CreatedBy        uuid.UUID   `json:"created_by"`
	MemberIDs        []uuid.UUID `json:"member_ids"`
}

type MessageCreatedPayload struct {
	ConversationID   uuid.UUID   `json:"conversation_id"`
	MessageID        uuid.UUID   `json:"message_id"`
	SenderID         uuid.UUID   `json:"sender_id"`
	ReplyToMessageID *uuid.UUID  `json:"reply_to_message_id"`
	CreatedAt        time.Time   `json:"created_at"`
	MemberIDs        []uuid.UUID `json:"member_ids"`
	ContentEnc       []byte      `json:"content_enc,omitempty"`
	Content          *string     `json:"content,omitempty"`
}

type MessageUpdatedPayload struct {
	ConversationID uuid.UUID   `json:"conversation_id"`
	MessageID      uuid.UUID   `json:"message_id"`
	SenderID       uuid.UUID   `json:"sender_id"`
	UpdatedAt      time.Time   `json:"updated_at"`
	MemberIDs      []uuid.UUID `json:"member_ids"`
	ContentEnc     []byte      `json:"content_enc,omitempty"`
	Content        *string     `json:"content,omitempty"`
}

type MessageDeletedPayload struct {
	ConversationID uuid.UUID   `json:"conversation_id"`
	MessageID      uuid.UUID   `json:"message_id"`
	DeletedBy      uuid.UUID   `json:"deleted_by"`
	MemberIDs      []uuid.UUID `json:"member_ids"`
}

type MessageReadPayload struct {
	ConversationID uuid.UUID   `json:"conversation_id"`
	ReaderID       uuid.UUID   `json:"reader_id"`
	MessageID      uuid.UUID   `json:"message_id"`
	MemberIDs      []uuid.UUID `json:"member_ids"`
}

// Для событий участников MemberIDs — все, кого нужно уведомить (включая затронутого пользователя).

type ConversationUpdatedPayload struct {
	ConversationID uuid.UUID   `json:"conversation_id"`
	UpdatedBy      uuid.UUID   `json:"updated_by"`
	Fields         []string    `json:"fields"`
	TargetUserID   *uuid.UUID  `json:"target_user_id,omitempty"`
	MemberIDs      []uuid.UUID `json:"member_ids"`
}

type ConversationDeletedPayload struct {
	ConversationID uuid.UUID   `json:"conversation_id"`
	DeletedBy      uuid.UUID   `json:"deleted_by"`
	MemberIDs      []uuid.UUID `json:"member_ids"`
}

type MemberAddedPayload struct {
	ConversationID uuid.UUID   `json:"conversation_id"`
	UserIDs        []uuid.UUID `json:"user_ids"`
	AddedBy        uuid.UUID   `json:"added_by"`
	MemberIDs      []uuid.UUID `json:"member_ids"`
}

type MemberRemovedPayload struct {
	ConversationID uuid.UUID   `json:"conversation_id"`
	UserID         uuid.UUID   `json:"user_id"`
	RemovedBy      uuid.UUID   `json:"removed_by"`
	MemberIDs      []uuid.UUID `json:"member_ids"`
}

// AdminIDs — администраторы, которых нужно уведомить о новой заявке.
type JoinRequestCreatedPayload struct {
	ConversationID uuid.UUID   `json:"conversation_id"`
	RequestID      uuid.UUID   `json:"request_id"`
	UserID         uuid.UUID   `json:"user_id"`
	AdminIDs       []uuid.UUID `json:"admin_ids"`
}

// MemberIDs — участники после одобрения (включая вступившего).
type JoinRequestApprovedPayload struct {
	ConversationID uuid.UUID   `json:"conversation_id"`
	RequestID      uuid.UUID   `json:"request_id"`
	UserID         uuid.UUID   `json:"user_id"`
	ApprovedBy     uuid.UUID   `json:"approved_by"`
	MemberIDs      []uuid.UUID `json:"member_ids"`
}
