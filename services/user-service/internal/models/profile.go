package models

import (
	"time"

	"github.com/google/uuid"
)

type Profile struct {
	ID        uuid.UUID  `json:"id"`
	Username  string     `json:"username"`
	FullName  string     `json:"full_name"`
	Bio       string     `json:"bio,omitempty"`
	AvatarURL string     `json:"avatar_url,omitempty"`
	BirthDate *time.Time `json:"birth_date,omitempty"`

	LastSeen  time.Time  `json:"last_seen"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`

	Settings *Settings `json:"settings,omitempty"`
}
