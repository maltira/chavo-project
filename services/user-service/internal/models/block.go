package models

import (
	"time"

	"github.com/google/uuid"
)

type Block struct {
	UserID        uuid.UUID `json:"user_id"`
	BlockedUserID uuid.UUID `json:"blocked_user_id"`
	CreatedAt     time.Time `json:"created_at"`
}

// BlockedEntry — запись для списка заблокированных (JOIN с профилем).
type BlockedEntry struct {
	BlockedUserID uuid.UUID `json:"blocked_user_id"`
	Username      string    `json:"username"`
	DisplayName   string    `json:"display_name"`
	AvatarURL     *string   `json:"avatar_url,omitempty"`
	BlockedAt     time.Time `json:"blocked_at"`
}
