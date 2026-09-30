package models

import (
	"time"

	"github.com/google/uuid"
)

type PasswordResetToken struct {
	UserID    uuid.UUID `json:"user_id"`
	TokenHash string    `json:"token_hash"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}
