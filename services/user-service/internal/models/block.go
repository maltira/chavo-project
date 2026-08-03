package models

import (
	"time"

	"github.com/google/uuid"
)

type Block struct {
	ID               uuid.UUID `json:"id"`
	ProfileID        uuid.UUID `json:"profile_id"`
	BlockedProfileID uuid.UUID `json:"blocked_profile_id"`
	CreatedAt        time.Time `json:"created_at"`

	Profile        *Profile `json:"profile,omitempty"`
	BlockedProfile *Profile `json:"blocked_profile,omitempty"`
}
