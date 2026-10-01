package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models/dto"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/service"
)

type ProfileHandler struct {
	svc service.ProfileService
	rdb *redis.Client
	log *zap.Logger
}

func NewProfileHandler(svc service.ProfileService, rdb *redis.Client, log *zap.Logger) *ProfileHandler {
	return &ProfileHandler{svc: svc, rdb: rdb, log: log}
}

// POST /users/me
func (h *ProfileHandler) CreateProfile(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}

	var req dto.CreateProfileRequest
	if !bindJSON(c, &req) {
		return
	}

	input := service.CreateProfileInput{
		Username:    req.Username,
		DisplayName: req.DisplayName,
		Bio:         req.Bio,
		AvatarURL:   req.AvatarURL,
	}

	if err := h.svc.Create(c.Request.Context(), userID, input); err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusCreated, dto.MessageResponse{Success: true, Message: "Профиль успешно создан"})
}

// GET /users/me
func (h *ProfileHandler) GetMe(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}

	profile, err := h.svc.FindByID(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, profile)
}

// PATCH /users/me
func (h *ProfileHandler) UpdateMe(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}

	var req dto.UpdateProfileRequest
	if !bindJSON(c, &req) {
		return
	}

	data := make(map[string]string)
	if req.Username != nil {
		data["username"] = *req.Username
	}
	if req.DisplayName != nil {
		data["display_name"] = *req.DisplayName
	}
	if req.Bio != nil {
		data["bio"] = *req.Bio
	}
	if req.AvatarURL != nil {
		data["avatar_url"] = *req.AvatarURL
	}

	if err := h.svc.Update(c.Request.Context(), userID, data); err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Success: true, Message: "Профиль успешно обновлён"})
}

// GET /users/:user_id
func (h *ProfileHandler) GetByID(c *gin.Context) {
	profileID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	profile, err := h.svc.FindByID(c.Request.Context(), profileID)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, profile)
}

// GET /users?q=...&limit=10&offset=0
func (h *ProfileHandler) Search(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		respondError(c, apperror.ErrIncorrectData, h.log)
		return
	}

	limit, offset := parsePagination(c, 8, 20)

	profiles, err := h.svc.GetAllBySearch(c.Request.Context(), q, limit, offset)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, profiles)
}

// — helpers —

func parseUserID(c *gin.Context, log *zap.Logger) (uuid.UUID, bool) {
	userID, err := uuid.Parse(c.GetHeader("X-User-ID"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, log)
		return uuid.Nil, false
	}
	return userID, true
}

func parsePagination(c *gin.Context, defaultLimit, maxLimit int) (limit, offset int) {
	limit, _ = strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultLimit)))
	offset, _ = strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit < 1 || limit > maxLimit {
		limit = defaultLimit
	}
	if offset < 0 {
		offset = 0
	}
	return
}
