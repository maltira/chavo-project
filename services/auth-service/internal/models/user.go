package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	TwoFactorEmail = "email"
	TwoFactorTOTP  = "totp"
)

type User struct {
	ID              uuid.UUID  `json:"id"`
	Email           string     `json:"email"`
	PasswordHash    string     `json:"-"`
	EmailVerified   bool       `json:"email_verified"`
	TwoFactorMethod string     `json:"two_factor_method"`
	TOTPSecretEnc   []byte     `json:"-"`
	TOTPVerifiedAt  *time.Time `json:"totp_verified_at,omitempty"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}
