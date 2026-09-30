package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/auth-service/internal/models/dto"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/service"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/utils"
)

type AuthHandler struct {
	auth service.AuthService
	ts   service.TokenService
	log  *zap.Logger
}

func NewAuthHandler(auth service.AuthService, ts service.TokenService, log *zap.Logger) *AuthHandler {
	return &AuthHandler{auth: auth, ts: ts, log: log}
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

	c.JSON(http.StatusCreated, dto.MessageResponse{
		Message: "Ссылка для подтверждения отправлена на указанную почту",
	})
}

func (h *AuthHandler) VerifyRegister(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:  http.StatusBadRequest,
			Error: "Токен верификации не указан",
		})
		return
	}

	if err := h.auth.VerifyRegister(c.Request.Context(), token); err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{
		Message: "Аккаунт успешно подтверждён",
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if !bindJSON(c, &req) {
		return
	}

	challengeID, err := h.auth.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.LoginChallengeResponse{
		ChallengeID: challengeID,
		Message:     "OTP-код отправлен на вашу почту",
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		utils.ClearAuthCookies(c)
		c.JSON(http.StatusOK, dto.MessageResponse{
			Message: "Вы вышли из аккаунта",
		})
		return
	}

	userIDStr := c.GetHeader("X-User-ID")
	userID, err := uuid.Parse(userIDStr)
	if err == nil {
		_ = h.ts.RevokeCurrent(c.Request.Context(), userID, refreshToken)
	}

	utils.ClearAuthCookies(c)
	c.JSON(http.StatusOK, dto.MessageResponse{
		Message: "Вы вышли из аккаунта",
	})
}

func (h *AuthHandler) RequestPasswordReset(c *gin.Context) {
	var req dto.ResetPasswordRequest
	if !bindJSON(c, &req) {
		return
	}

	if err := h.auth.RequestPasswordReset(c.Request.Context(), req.Email); err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{
		Message: "Если аккаунт существует, ссылка для сброса пароля отправлена на почту",
	})
}

func (h *AuthHandler) ConfirmPasswordReset(c *gin.Context) {
	var req dto.ResetPasswordConfirmRequest
	if !bindJSON(c, &req) {
		return
	}

	if err := h.auth.ConfirmPasswordReset(c.Request.Context(), req.Token, req.Password); err != nil {
		respondError(c, err, h.log)
		return
	}

	utils.ClearAuthCookies(c)
	c.JSON(http.StatusOK, dto.MessageResponse{
		Message: "Пароль успешно изменён. Выполните вход с новым паролем",
	})
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, ok := utils.GetXUserID(c)
	if !ok {
		return
	}

	var req dto.ChangePasswordRequest
	if !bindJSON(c, &req) {
		return
	}

	if err := h.auth.ChangePassword(c.Request.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
		respondError(c, err, h.log)
		return
	}

	utils.ClearAuthCookies(c)
	c.JSON(http.StatusOK, dto.MessageResponse{
		Message: "Пароль успешно изменён. Выполните вход с новым паролем",
	})
}
