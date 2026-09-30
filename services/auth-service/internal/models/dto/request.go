package dto

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type VerifyOTPRequest struct {
	ChallengeID string `json:"challenge_id" binding:"required"`
	Code        string `json:"code" binding:"required,len=6"`
}

type ResendOTPRequest struct {
	ChallengeID string `json:"challenge_id" binding:"required"`
}

type ResetPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ResetPasswordConfirmRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8,max=72"`
}
