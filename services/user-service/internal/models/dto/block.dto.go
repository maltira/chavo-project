package dto

import "github.com/google/uuid"

type BlockRequest struct {
	ProfileID        uuid.UUID `json:"profile_id" binding:"required"`
	BlockedProfileID uuid.UUID `json:"blocked_profile_id" binding:"required,nefield=ProfileID"`
}
