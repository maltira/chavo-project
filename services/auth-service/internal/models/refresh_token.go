package models

import (
	"time"

	"github.com/google/uuid"
)

type RefreshToken struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	Token     string    `json:"-" db:"token"`
	AccessJTI string    `json:"-" db:"access_jti"`

	IP        string `json:"ip" db:"ip"`
	UserAgent string `json:"user_agent" db:"user_agent"`
	Device    string `json:"device" db:"device"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
}
