package models

import (
	"time"

	"github.com/google/uuid"
)

type RefreshToken struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	TokenHash string    `json:"-"`

	DeviceName *string `json:"device_name,omitempty"`
	UserAgent  *string `json:"user_agent,omitempty"`
	IPAddress  *string `json:"ip_address,omitempty"`

	ExpiresAt time.Time  `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}
