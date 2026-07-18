package dto

import "github.com/google/uuid"

type ErrorResponse struct {
	Code  int    `json:"code"`
	Error string `json:"error"`
}

type MessageResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type OTPSentResponse struct {
	UserID  uuid.UUID `json:"user_id"`
	Message string    `json:"message"`
}

type LoginResponse struct {
	UserID      uuid.UUID `json:"user_id"`
	Email       string    `json:"email"`
	AccessToken string    `json:"access_token"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}
