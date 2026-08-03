package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models/dto"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/service"
)

type SettingsHandler struct {
	sc  service.SettingsService
	log *zap.Logger
}

func NewSettingsHandler(sc service.SettingsService, log *zap.Logger) *SettingsHandler {
	return &SettingsHandler{sc: sc, log: log}
}

// GetSettings возвращает настройки текущего пользователя.
func (h *SettingsHandler) GetSettings(c *gin.Context) {
	userID, err := uuid.Parse(c.GetHeader("X-User-ID"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	settings, err := h.sc.GetSettings(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, settings)
}

// UpdateVisibleStatus обновляет отображение статуса в сети.
func (h *SettingsHandler) UpdateVisibleStatus(c *gin.Context) {
	userID, err := uuid.Parse(c.GetHeader("X-User-ID"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	visible := c.Query("visible")
	if visible == "" {
		respondError(c, apperror.ErrIncorrectData, h.log)
		return
	}

	err = h.sc.UpdateVisibleStatus(c.Request.Context(), userID, visible == "true")
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Success: true, Message: "Отображение статуса успешно изменено"})
}

// UpdateVisibleBirthDate обновляет отображение даты рождения.
func (h *SettingsHandler) UpdateVisibleBirthDate(c *gin.Context) {
	userID, err := uuid.Parse(c.GetHeader("X-User-ID"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	visible := c.Query("visible")
	if visible == "" {
		respondError(c, apperror.ErrIncorrectData, h.log)
		return
	}

	err = h.sc.UpdateVisibleBirthDate(c.Request.Context(), userID, visible == "true" || visible == "all")
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Success: true, Message: "Отображение даты рождения успешно изменено"})
}
