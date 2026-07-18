package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/auth-service/config"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/dto"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/service"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/utils"
)

type OtpHandler struct {
	otp  service.OtpService
	auth service.AuthService
	ts   service.TokenService
	cfg  *config.Config
	log  *zap.Logger
}

func NewOtpHandler(otp service.OtpService, auth service.AuthService, ts service.TokenService, cfg *config.Config, log *zap.Logger) *OtpHandler {
	return &OtpHandler{otp: otp, auth: auth, ts: ts, cfg: cfg, log: log}
}

func (h *OtpHandler) VerifyLoginOTP(c *gin.Context) {
	var req dto.VerifyLoginOTPRequest
	if !bindJSON(c, &req) {
		return
	}

	ctx := c.Request.Context()

	_, err := h.otp.VerifyAndMark(ctx, req.UserID, req.Code, "login")
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	user, err := h.auth.FindByID(ctx, req.UserID)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	ip := c.ClientIP()
	userAgent := c.Request.UserAgent()
	device := utils.ParseDeviceInfo(userAgent)

	access, refresh, err := h.ts.GenerateTokens(ctx, user.ID, ip, userAgent, device)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	utils.SetAuthCookies(c, refresh, int(h.cfg.RefreshTokenDuration.Seconds()))
	c.JSON(http.StatusOK, dto.LoginResponse{
		UserID:      user.ID,
		Email:       user.Email,
		AccessToken: access,
	})
}

func (h *OtpHandler) VerifyDeleteAccountOTP(c *gin.Context) {
	var req dto.VerifyDeleteOTPRequest
	if !bindJSON(c, &req) {
		return
	}

	ctx := c.Request.Context()

	_, err := h.otp.VerifyAndMark(ctx, req.UserID, req.Code, "account_delete")
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	user, err := h.auth.FindByID(ctx, req.UserID)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	if err = h.auth.SoftDeleteUser(ctx, user.ID, user.Email, req.Reason, "user"); err != nil {
		respondError(c, err, h.log)
		return
	}

	_ = h.ts.RevokeAll(ctx, user.ID, nil)
	utils.ClearAuthCookies(c)

	c.JSON(http.StatusOK, dto.MessageResponse{
		Success: true,
		Message: "Аккаунт был удалён",
	})
}

func (h *OtpHandler) VerifyChangeMailOTP(c *gin.Context) {
	var req dto.VerifyChangeEmailRequest
	if !bindJSON(c, &req) {
		return
	}

	ctx := c.Request.Context()

	_, err := h.otp.VerifyAndMark(ctx, req.UserID, req.Code, "email_change")
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	if err := h.auth.ApplyEmailChange(ctx, req.UserID, req.NewEmail); err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{
		Success: true,
		Message: "Почта успешно изменена",
	})
}

func (h *OtpHandler) VerifyChangePasswordOTP(c *gin.Context) {
	var req dto.VerifyChangePassRequest
	if !bindJSON(c, &req) {
		return
	}

	ctx := c.Request.Context()

	_, err := h.otp.VerifyAndMark(ctx, req.UserID, req.Code, "password_change")
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	if err := h.auth.ApplyPasswordChange(ctx, req.UserID, req.Password, req.NewPassword); err != nil {
		respondError(c, err, h.log)
		return
	}

	utils.ClearAuthCookies(c)
	c.JSON(http.StatusOK, dto.MessageResponse{
		Success: true,
		Message: "Пароль успешно изменён",
	})
}
