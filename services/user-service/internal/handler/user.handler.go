package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models/dto"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/service"
	"github.com/maltira/chavo-project-backend/services/user-service/pkg/websocket"
)

type ProfileHandler struct {
	sc  service.ProfileService
	log *zap.Logger
}

func NewProfileHandler(sc service.ProfileService, log *zap.Logger) *ProfileHandler {
	return &ProfileHandler{sc: sc, log: log}
}

// CreateProfile создаёт профиль пользователя (межсервисный вызов).
func (h *ProfileHandler) CreateProfile(c *gin.Context) {
	var req dto.CreateProfileRequest
	if !bindJSON(c, &req) {
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	if err = h.sc.Create(c.Request.Context(), userID); err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusCreated, dto.MessageResponse{Success: true, Message: "Профиль успешно создан"})
}

// GetCurrentProfile возвращает профиль текущего авторизованного пользователя.
func (h *ProfileHandler) GetCurrentProfile(c *gin.Context) {
	userID, err := uuid.Parse(c.GetHeader("X-User-ID"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	profile, err := h.sc.FindByID(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, profile)
}

// GetProfileByID возвращает публичный профиль по ID.
func (h *ProfileHandler) GetProfileByID(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	user, err := h.sc.FindByID(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, user)
}

// GetProfilesByQuery ищет профили по поисковому запросу.
func (h *ProfileHandler) GetProfilesByQuery(c *gin.Context) {
	query := c.Query("q")
	l := c.DefaultQuery("limit", "10")

	if query == "" {
		respondError(c, apperror.ErrIncorrectData, h.log)
		return
	}

	limit, err := strconv.Atoi(l)
	if err != nil || limit < 1 || limit > 20 {
		limit = 10
	}

	profiles, err := h.sc.GetAllBySearch(c.Request.Context(), query, limit)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, profiles)
}

// UpdateProfile обновляет данные профиля текущего пользователя.
func (h *ProfileHandler) UpdateProfile(c *gin.Context) {
	userID, err := uuid.Parse(c.GetHeader("X-User-ID"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	var req map[string]any
	if !bindJSON(c, &req) {
		return
	}

	if err = h.sc.Update(c.Request.Context(), userID, req); err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Success: true, Message: "Новые данные успешно сохранены"})
}

// IsUsernameFree проверяет доступность username.
func (h *ProfileHandler) IsUsernameFree(c *gin.Context) {
	username := c.Query("u")
	if len(username) < 4 || len(username) > 16 {
		respondError(c, apperror.ErrInvalidUsername, h.log)
		return
	}

	isFree, err := h.sc.IsUsernameFree(c.Request.Context(), username)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, isFree)
}

// GetUserStatus проверяет статус онлайн пользователя.
func (h *ProfileHandler) GetUserStatus(c *gin.Context) {
	profileID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	profile, err := h.sc.FindByID(c.Request.Context(), profileID)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	if profile.Settings != nil && !profile.Settings.ShowOnlineStatus {
		c.JSON(http.StatusOK, dto.ProfileStatusResponse{
			Online:   false,
			LastSeen: nil,
		})
		return
	}

	isOnline := websocket.IsClientOnline(profileID)
	c.JSON(http.StatusOK, dto.ProfileStatusResponse{
		Online:   isOnline,
		LastSeen: &profile.LastSeen,
	})
}
