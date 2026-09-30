package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/auth-service/config"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/models/dto"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/service"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/utils"
)

type RefreshHandler struct {
	ts  service.TokenService
	cfg *config.Config
	log *zap.Logger
}

func NewRefreshHandler(ts service.TokenService, cfg *config.Config, log *zap.Logger) *RefreshHandler {
	return &RefreshHandler{ts: ts, cfg: cfg, log: log}
}

func (h *RefreshHandler) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
			Code:  http.StatusUnauthorized,
			Error: "Необходима авторизация",
		})
		return
	}

	ip := c.ClientIP()
	userAgent := c.Request.UserAgent()
	device := utils.ParseDeviceInfo(userAgent)

	access, newRefresh, err := h.ts.Refresh(c.Request.Context(), refreshToken, ip, userAgent, device)
	if err != nil {
		utils.ClearAuthCookies(c)
		respondError(c, err, h.log)
		return
	}

	utils.SetAuthCookies(c, newRefresh, int(h.cfg.RefreshTokenDuration.Seconds()))
	c.JSON(http.StatusOK, dto.TokenResponse{
		AccessToken: access,
		TokenType:   "Bearer",
	})
}

func (h *RefreshHandler) ListSessions(c *gin.Context) {
	userID, ok := utils.GetXUserID(c)
	if !ok {
		return
	}

	sessions, err := h.ts.ListActiveSessions(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	resp := make([]dto.SessionResponse, 0, len(sessions))
	for _, s := range sessions {
		resp = append(resp, dto.SessionResponse{
			ID:         s.ID,
			DeviceName: s.DeviceName,
			UserAgent:  s.UserAgent,
			IPAddress:  s.IPAddress,
			CreatedAt:  s.CreatedAt,
			ExpiresAt:  s.ExpiresAt,
		})
	}

	c.JSON(http.StatusOK, resp)
}

func (h *RefreshHandler) TerminateSession(c *gin.Context) {
	userID, ok := utils.GetXUserID(c)
	if !ok {
		return
	}

	sessionID, err := uuid.Parse(c.Param("session_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:  http.StatusBadRequest,
			Error: "Некорректный UUID сессии",
		})
		return
	}

	if err = h.ts.RevokeByID(c.Request.Context(), userID, sessionID); err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{
		Message: "Сессия завершена",
	})
}
