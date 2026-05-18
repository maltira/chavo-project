package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID         uuid.UUID `json:"id" db:"id"`
	Email      string    `json:"email" db:"email"`
	Password   string    `json:"-" db:"password"`
	IsVerified bool      `json:"is_verified" db:"is_verified"`

	DeletedAt      *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
	DeletionReason *string    `json:"deletion_reason,omitempty" db:"deletion_reason"`
	DeletedBy      *string    `json:"deleted_by,omitempty" db:"deleted_by"` // "user" или "system"

	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	PasswordUpdatedAt time.Time `json:"password_updated_at" db:"password_updated_at"`
	EmailUpdatedAt    time.Time `json:"email_updated_at" db:"email_updated_at"`
}
