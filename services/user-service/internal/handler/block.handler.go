package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models/dto"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/service"
	"github.com/maltira/chavo-project-backend/services/user-service/pkg/utils"
)

type BlockHandler struct {
	sc  service.BlockService
	rdb *redis.Client
	log *zap.Logger
}

func NewBlockHandler(sc service.BlockService, rdb *redis.Client, log *zap.Logger) *BlockHandler {
	return &BlockHandler{sc: sc, rdb: rdb, log: log}
}

// GetAllBlocks возвращает список заблокированных пользователей.
func (h *BlockHandler) GetAllBlocks(c *gin.Context) {
	userID, err := uuid.Parse(c.GetHeader("X-User-ID"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	blocks, err := h.sc.GetAllBlocks(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusOK, blocks)
}

// IsBlocked проверяет, заблокирован ли текущий пользователь.
func (h *BlockHandler) IsBlocked(c *gin.Context) {
	userID, err := uuid.Parse(c.GetHeader("X-User-ID"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	targetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	isBlocked, err := h.sc.IsBlock(c.Request.Context(), targetID, userID)
	if err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusOK, isBlocked)
}

// BlockUser блокирует пользователя.
func (h *BlockHandler) BlockUser(c *gin.Context) {
	userID, err := uuid.Parse(c.GetHeader("X-User-ID"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	blockUUID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	blockedProfile, err := h.sc.BlockUser(c.Request.Context(), userID, blockUUID)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	if err = utils.PublishBlockEvent(h.rdb, userID, blockUUID, true); err != nil {
		h.log.Error("Failed to publish block event", zap.Error(err))
	}

	c.JSON(http.StatusOK, blockedProfile)
}

// UnblockUser разблокирует пользователя.
func (h *BlockHandler) UnblockUser(c *gin.Context) {
	userID, err := uuid.Parse(c.GetHeader("X-User-ID"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	blockedUUID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	if err = h.sc.UnblockUser(c.Request.Context(), userID, blockedUUID); err != nil {
		respondError(c, err, h.log)
		return
	}

	if err = utils.PublishBlockEvent(h.rdb, userID, blockedUUID, false); err != nil {
		h.log.Error("Failed to publish unblock event", zap.Error(err))
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Success: true, Message: "Пользователь удалён из черного списка"})
}
