package dto

import (
	"time"

	"github.com/google/uuid"
)

type CreateProfileRequest struct {
	Username    string  `json:"username"     binding:"required"`
	DisplayName string  `json:"display_name" binding:"required"`
	Bio         *string `json:"bio"`
	AvatarURL   *string `json:"avatar_url"`
}

type UpdateProfileRequest struct {
	Username    *string `json:"username"`
	DisplayName *string `json:"display_name"`
	Bio         *string `json:"bio"`
	AvatarURL   *string `json:"avatar_url"`
}

type PresenceRequest struct {
	UserIDs []uuid.UUID `json:"user_ids" binding:"required,min=1"`
}

type PresenceEntry struct {
	UserID     uuid.UUID  `json:"user_id"`
	Online     bool       `json:"online"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
}
