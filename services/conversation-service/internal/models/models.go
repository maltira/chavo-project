package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	ConversationDirect = "direct"
	ConversationGroup  = "group"

	VisibilityPublic  = "public"
	VisibilityPrivate = "private"

	RoleMember = "member"
	RoleAdmin  = "admin"

	JoinPending  = "pending"
	JoinApproved = "approved"
	JoinRejected = "rejected"
)

type Conversation struct {
	ID               uuid.UUID `json:"id"`
	ConversationType string    `json:"conversation_type"`
	Visibility       string    `json:"visibility"`
	Name             *string   `json:"name"`
	Description      *string   `json:"description"`
	AvatarURL        *string   `json:"avatar_url"`
	LastMessageAt    time.Time `json:"last_message_at"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Member struct {
	ConversationID    uuid.UUID  `json:"conversation_id"`
	UserID            uuid.UUID  `json:"user_id"`
	Role              string     `json:"role"`
	LastReadMessageID *uuid.UUID `json:"last_read_message_id"`
	JoinedAt          time.Time  `json:"joined_at"`
}

// Message — расшифрованное сообщение; Content == nil у удалённых.
type Message struct {
	ID               uuid.UUID  `json:"id"`
	ConversationID   uuid.UUID  `json:"conversation_id"`
	SenderID         uuid.UUID  `json:"sender_id"`
	Content          *string    `json:"content"`
	ReplyToMessageID *uuid.UUID `json:"reply_to_message_id"`
	IsEdited         bool       `json:"is_edited"`
	IsDeleted        bool       `json:"is_deleted"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        *time.Time `json:"updated_at"`
}

type JoinRequest struct {
	ID             uuid.UUID `json:"id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	UserID         uuid.UUID `json:"user_id"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ConversationSummary struct {
	Conversation
	MyRole      string     `json:"my_role"`
	PeerID      *uuid.UUID `json:"peer_id"`
	LastMessage *Message   `json:"last_message"`
	UnreadCount int        `json:"unread_count"`
}

// InviteInfo — краткая информация о приватной группе по invite-ссылке.
type InviteInfo struct {
	ConversationID uuid.UUID `json:"conversation_id"`
	Name           *string   `json:"name"`
	Description    *string   `json:"description"`
	AvatarURL      *string   `json:"avatar_url"`
	MembersCount   int       `json:"members_count"`
	IsMember       bool      `json:"is_member"`
	RequestStatus  *string   `json:"request_status"`
}

// PublicGroup — краткая карточка публичной группы для поиска.
type PublicGroup struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Description  *string   `json:"description"`
	AvatarURL    *string   `json:"avatar_url"`
	MembersCount int       `json:"members_count"`
	IsMember     bool      `json:"is_member"`
}
