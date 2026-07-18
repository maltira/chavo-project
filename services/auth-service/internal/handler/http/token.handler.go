package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/auth-service/config"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/dto"
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

	access, newRefresh, err := h.ts.Refresh(c.Request.Context(), refreshToken)
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

func (h *RefreshHandler) TerminateSession(c *gin.Context) {
	userID, err := uuid.Parse(c.GetHeader("X-User-ID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:  http.StatusBadRequest,
			Error: "Некорректный UUID пользователя",
		})
		return
	}

	tokenID, err := uuid.Parse(c.Param("token_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:  http.StatusBadRequest,
			Error: "Некорректный UUID сессии",
		})
		return
	}

	if err = h.ts.RevokeByID(c.Request.Context(), userID, tokenID); err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{
		Success: true,
		Message: "Сессия завершена",
	})
}

func (h *RefreshHandler) ListSessions(c *gin.Context) {
	userID, err := uuid.Parse(c.GetHeader("X-User-ID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:  http.StatusBadRequest,
			Error: "Некорректный UUID пользователя",
		})
		return
	}

	sessions, err := h.ts.ListActiveSessions(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, sessions)
}
