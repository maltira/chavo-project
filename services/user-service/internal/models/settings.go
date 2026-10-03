package models

import (
	"time"

	"github.com/google/uuid"
)

type Settings struct {
	UserID            uuid.UUID `json:"user_id"`
	AllowGroupInvites bool      `json:"allow_group_invites"`
	ShowOnlineStatus  bool      `json:"show_online_status"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
