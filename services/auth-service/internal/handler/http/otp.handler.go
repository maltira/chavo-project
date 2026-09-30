package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/auth-service/config"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/models/dto"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/service"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/utils"
)

type OtpHandler struct {
	otpSvc   service.OtpService
	tokenSvc service.TokenService
	cfg      *config.Config
	log      *zap.Logger
}

func NewOtpHandler(
	otpSvc service.OtpService,
	tokenSvc service.TokenService,
	cfg *config.Config,
	log *zap.Logger,
) *OtpHandler {
	return &OtpHandler{
		otpSvc:   otpSvc,
		tokenSvc: tokenSvc,
		cfg:      cfg,
		log:      log,
	}
}

func (h *OtpHandler) VerifyOTP(c *gin.Context) {
	var req dto.VerifyOTPRequest
	if !bindJSON(c, &req) {
		return
	}

	userID, err := h.otpSvc.VerifyChallenge(c.Request.Context(), req.ChallengeID, req.Code)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	ip := c.ClientIP()
	userAgent := c.Request.UserAgent()
	device := utils.ParseDeviceInfo(userAgent)

	accessToken, refreshToken, err := h.tokenSvc.GenerateTokens(c.Request.Context(), userID, ip, userAgent, device)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	utils.SetAuthCookies(c, refreshToken, int(h.cfg.RefreshTokenDuration.Seconds()))

	c.JSON(http.StatusOK, dto.TokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
	})
}

func (h *OtpHandler) ResendOTP(c *gin.Context) {
	var req dto.ResendOTPRequest
	if !bindJSON(c, &req) {
		return
	}

	if err := h.otpSvc.ResendChallenge(c.Request.Context(), req.ChallengeID); err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{
		Message: "Новый OTP-код отправлен на вашу почту",
	})
}
