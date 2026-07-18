package dto

import "github.com/google/uuid"

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email,max=254"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ResetPasswordRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

type ChangePasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

type ChangeEmailRequest struct {
	NewEmail string `json:"new_email" binding:"required,email,max=254"`
}

type VerifyLoginOTPRequest struct {
	UserID uuid.UUID `json:"user_id" binding:"required"`
	Code   string    `json:"code" binding:"required,len=6"`
}

type VerifyDeleteOTPRequest struct {
	UserID uuid.UUID `json:"user_id" binding:"required"`
	Code   string    `json:"code" binding:"required,len=6"`
	Reason string    `json:"reason" binding:"required,max=500"`
}

type VerifyChangeEmailRequest struct {
	UserID   uuid.UUID `json:"user_id" binding:"required"`
	Code     string    `json:"code" binding:"required,len=6"`
	NewEmail string    `json:"new_email" binding:"required,email,max=254"`
}

type VerifyChangePassRequest struct {
	UserID      uuid.UUID `json:"user_id" binding:"required"`
	Code        string    `json:"code" binding:"required,len=6"`
	Password    string    `json:"password" binding:"required"`
	NewPassword string    `json:"new_password" binding:"required,min=8,max=72"`
}
