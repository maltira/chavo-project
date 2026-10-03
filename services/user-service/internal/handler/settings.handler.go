package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/models/dto"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/service"
)

type SettingsHandler struct {
	svc service.SettingsService
	log *zap.Logger
}

func NewSettingsHandler(svc service.SettingsService, log *zap.Logger) *SettingsHandler {
	return &SettingsHandler{svc: svc, log: log}
}

// GET /users/me/settings
func (h *SettingsHandler) GetSettings(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}

	settings, err := h.svc.GetSettings(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, settings)
}

// PATCH /users/me/settings
func (h *SettingsHandler) UpdateSettings(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}

	var req dto.UpdateSettingsRequest
	if !bindJSON(c, &req) {
		return
	}

	data := make(map[string]any)
	if req.AllowGroupInvites != nil {
		data["allow_group_invites"] = *req.AllowGroupInvites
	}
	if req.ShowOnlineStatus != nil {
		data["show_online_status"] = *req.ShowOnlineStatus
	}

	if err := h.svc.UpdateSettings(c.Request.Context(), userID, data); err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Success: true, Message: "Настройки успешно обновлены"})
}
