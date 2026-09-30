package dto

import (
	"time"

	"github.com/google/uuid"
)

type ErrorResponse struct {
	Code  int    `json:"code"`
	Error string `json:"error"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type LoginChallengeResponse struct {
	ChallengeID string `json:"challenge_id"`
	Message     string `json:"message"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type SessionResponse struct {
	ID         uuid.UUID `json:"id"`
	DeviceName *string   `json:"device_name,omitempty"`
	UserAgent  *string   `json:"user_agent,omitempty"`
	IPAddress  *string   `json:"ip_address,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}
