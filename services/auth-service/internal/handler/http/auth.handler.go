package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/auth-service/internal/dto"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/service"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/utils"
)

type AuthHandler struct {
	auth service.AuthService
	otp  service.OtpService
	ts   service.TokenService
	log  *zap.Logger
}

func NewAuthHandler(auth service.AuthService, otp service.OtpService, ts service.TokenService, log *zap.Logger) *AuthHandler {
	return &AuthHandler{auth: auth, otp: otp, ts: ts, log: log}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if !bindJSON(c, &req) {
		return
	}

	if err := h.auth.Register(c.Request.Context(), req.Email, req.Password); err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{
		Success: true,
		Message: "Ссылка для подтверждения отправлена",
	})
}

func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:  http.StatusBadRequest,
			Error: "Некорректные входные данные",
		})
		return
	}

	if err := h.auth.VerifyNewAccount(c.Request.Context(), token); err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{
		Success: true,
		Message: "Аккаунт успешно подтвержден",
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if !bindJSON(c, &req) {
		return
	}

	ctx := c.Request.Context()

	id, err := h.auth.Login(ctx, req.Email, req.Password)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	if err = h.otp.SendOTP(ctx, id, req.Email, "login"); err != nil {
		h.log.Error("Failed to send login OTP", zap.Error(err))
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.OTPSentResponse{
		UserID:  id,
		Message: "OTP-код отправлен на указанную почту",
	})
}

func (h *AuthHandler) LogoutCurrent(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		// No cookie — just clear and succeed (idempotent).
		utils.ClearAuthCookies(c)
		c.JSON(http.StatusOK, dto.MessageResponse{
			Success: true,
			Message: "Вы вышли из аккаунта",
		})
		return
	}

	if err = h.ts.RevokeByToken(c.Request.Context(), refreshToken); err != nil {
		h.log.Error("Failed to revoke token on logout", zap.Error(err))
		respondError(c, err, h.log)
		return
	}

	utils.ClearAuthCookies(c)
	c.JSON(http.StatusOK, dto.MessageResponse{
		Success: true,
		Message: "Вы вышли из аккаунта",
	})
}

func (h *AuthHandler) LogoutAll(c *gin.Context) {
	userID, err := uuid.Parse(c.GetHeader("X-User-ID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:  http.StatusBadRequest,
			Error: "Некорректный UUID пользователя",
		})
		return
	}

	refreshToken, _ := c.Cookie("refresh_token")
	var exclude *string
	if refreshToken != "" {
		exclude = &refreshToken
	}

	if err = h.ts.RevokeAll(c.Request.Context(), userID, exclude); err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{
		Success: true,
		Message: "Выполнен выход со всех устройств",
	})
}

func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req dto.ForgotPasswordRequest
	if !bindJSON(c, &req) {
		return
	}

	if err := h.auth.ForgotPassword(c.Request.Context(), req.Email); err != nil {
		respondError(c, err, h.log)
		return
	}

	// Always return success — don't reveal whether the email exists.
	c.JSON(http.StatusOK, dto.MessageResponse{
		Success: true,
		Message: "Пройдите по ссылке в письме, чтобы сменить пароль",
	})
}

func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req dto.ResetPasswordRequest
	if !bindJSON(c, &req) {
		return
	}

	tokenHash := utils.HashSHA256(req.Token)
	if err := h.auth.ResetPassword(c.Request.Context(), tokenHash, req.Password); err != nil {
		respondError(c, err, h.log)
		return
	}

	utils.ClearAuthCookies(c)
	c.JSON(http.StatusOK, dto.MessageResponse{
		Success: true,
		Message: "Пароль изменён",
	})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID, err := uuid.Parse(c.GetHeader("X-User-ID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:  http.StatusBadRequest,
			Error: "Передан невалидный UUID пользователя",
		})
		return
	}

	user, err := h.auth.FindByID(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) ChangePass(c *gin.Context) {
	userID, err := uuid.Parse(c.GetHeader("X-User-ID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:  http.StatusBadRequest,
			Error: "Передан невалидный UUID пользователя",
		})
		return
	}

	var req dto.ChangePasswordRequest
	if !bindJSON(c, &req) {
		return
	}

	ctx := c.Request.Context()

	user, err := h.auth.ValidatePasswordChange(ctx, userID, req.Password)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	if err = h.otp.SendOTP(ctx, user.ID, user.Email, "password_change"); err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.OTPSentResponse{
		UserID:  user.ID,
		Message: "OTP-код отправлен на вашу почту",
	})
}

func (h *AuthHandler) ChangeEmail(c *gin.Context) {
	userID, err := uuid.Parse(c.GetHeader("X-User-ID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:  http.StatusBadRequest,
			Error: "Передан невалидный UUID пользователя",
		})
		return
	}

	var req dto.ChangeEmailRequest
	if !bindJSON(c, &req) {
		return
	}

	ctx := c.Request.Context()

	user, err := h.auth.ValidateEmailChange(ctx, userID, req.NewEmail)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	if err = h.otp.SendOTP(ctx, user.ID, req.NewEmail, "email_change"); err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.OTPSentResponse{
		UserID:  user.ID,
		Message: "OTP-код отправлен на указанный адрес",
	})
}

func (h *AuthHandler) DeleteAccount(c *gin.Context) {
	userID, err := uuid.Parse(c.GetHeader("X-User-ID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:  http.StatusBadRequest,
			Error: "Передан невалидный UUID пользователя",
		})
		return
	}

	user, err := h.auth.FindByID(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	if err = h.otp.SendOTP(c.Request.Context(), user.ID, user.Email, "account_delete"); err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.OTPSentResponse{
		UserID:  user.ID,
		Message: "OTP-код отправлен на вашу почту",
	})
}
