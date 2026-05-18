package models

import (
	"time"

	"github.com/google/uuid"
)

type OTPCode struct {
	ID       uuid.UUID `json:"id" db:"id"`
	UserID   uuid.UUID `json:"user_id" db:"user_id"`
	Code     string    `json:"-" db:"code"`
	CodeType string    `json:"code_type" db:"code_type"`
	IsUsed   bool      `json:"is_used" db:"is_used"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
}
