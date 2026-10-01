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

type BlockHandler struct {
	svc service.BlockService
	log *zap.Logger
}

func NewBlockHandler(svc service.BlockService, log *zap.Logger) *BlockHandler {
	return &BlockHandler{svc: svc, log: log}
}

// GET /users/me/blocked?limit=&offset=
func (h *BlockHandler) GetBlocked(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}

	limit, offset := parsePagination(c, 20, 100)

	items, err := h.svc.GetBlockedUsers(c.Request.Context(), userID, limit, offset)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.BlockedListResponse{
		Items:  items,
		Limit:  limit,
		Offset: offset,
	})
}

// GET /users/:user_id/block-status
func (h *BlockHandler) GetBlockStatus(c *gin.Context) {
	myID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}

	targetID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	blockedByMe, blockedByThem, err := h.svc.GetBlockStatus(c.Request.Context(), myID, targetID)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.BlockStatusResponse{
		BlockedByMe:   blockedByMe,
		BlockedByThem: blockedByThem,
	})
}

// POST /users/:user_id/block
func (h *BlockHandler) BlockUser(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}

	targetID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	if err = h.svc.BlockUser(c.Request.Context(), userID, targetID); err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Success: true, Message: "Пользователь заблокирован"})
}

// DELETE /users/:user_id/block
func (h *BlockHandler) UnblockUser(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}

	targetID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	if err = h.svc.UnblockUser(c.Request.Context(), userID, targetID); err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Success: true, Message: "Пользователь разблокирован"})
}
