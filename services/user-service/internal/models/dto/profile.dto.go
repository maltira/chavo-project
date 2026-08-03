package dto

import (
	"time"
)

type CreateProfileRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

type UpdateProfileRequest struct {
	Username  *string    `json:"username" binding:"omitempty,min=4,max=16"`
	FullName  *string    `json:"full_name" binding:"omitempty,min=1,max=100"`
	Bio       *string    `json:"bio" binding:"omitempty,max=500"`
	AvatarURL *string    `json:"avatar_url" binding:"omitempty,url"`
	BirthDate *time.Time `json:"birth_date"`
}

type ProfileStatusResponse struct {
	Online   bool       `json:"online"`
	LastSeen *time.Time `json:"last_seen,omitempty"`
}
