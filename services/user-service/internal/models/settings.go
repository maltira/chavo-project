package models

import "github.com/google/uuid"

type Settings struct {
	ID               uuid.UUID `json:"id"`
	ProfileID        uuid.UUID `json:"profile_id"`
	ShowOnlineStatus bool      `json:"show_online_status"`
	ShowBirthDate    bool      `json:"show_birth_date"`
}
