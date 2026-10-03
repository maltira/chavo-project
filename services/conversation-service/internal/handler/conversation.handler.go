package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models/dto"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
)

type ConversationHandler struct {
	svc service.ConversationService
	log *zap.Logger
}

func NewConversationHandler(svc service.ConversationService, log *zap.Logger) *ConversationHandler {
	return &ConversationHandler{svc: svc, log: log}
}

// GET /conversations?limit=&offset=
func (h *ConversationHandler) List(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}
	limit, offset := parseLimit(c, 30, 100), parseOffset(c)

	items, err := h.svc.List(c.Request.Context(), userID, limit, offset)
	if err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusOK, dto.ConversationListResponse{Items: items, Limit: limit, Offset: offset})
}

// GET /conversations/:id
func (h *ConversationHandler) Get(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}
	convID, ok := parseUUIDParam(c, "id", h.log)
	if !ok {
		return
	}

	item, err := h.svc.Get(c.Request.Context(), userID, convID)
	if err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusOK, item)
}

// GET /conversations/search?q=&limit=&offset=
func (h *ConversationHandler) SearchGroups(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}
	limit, offset := parseLimit(c, 8, 100), parseOffset(c)

	items, err := h.svc.SearchPublicGroups(c.Request.Context(), userID, c.Query("q"), limit, offset)
	if err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusOK, dto.PublicGroupListResponse{Items: items, Limit: limit, Offset: offset})
}
