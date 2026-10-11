package models

import (
	"time"

	"github.com/google/uuid"
)

type Profile struct {
	UserID      uuid.UUID  `json:"user_id"`
	Username    string     `json:"username"`
	DisplayName string     `json:"display_name"`
	Bio         *string    `json:"bio,omitempty"`
	AvatarURL   *string    `json:"avatar_url,omitempty"`
	LastSeenAt  time.Time  `json:"last_seen_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"-"`
	Status      *Status    `json:"status,omitempty"`
	// AvatarShape - фигура позади аватара; nil - фигуры нет
	AvatarShape *AvatarShape `json:"avatar_shape,omitempty"`
}

type Status struct {
	Icon       string
	Text       string
	Background string
	Foreground string
	ExpiresAt  *time.Time
}

// PresenceBatchRow — строка для батч-запроса presence (DB часть).
type PresenceBatchRow struct {
	UserID           uuid.UUID
	LastSeenAt       time.Time
	ShowOnlineStatus bool
}

type AvatarShape struct {
	Shape string
	Color string
}
