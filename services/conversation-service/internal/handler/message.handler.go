package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models/dto"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
)

type MessageHandler struct {
	svc service.MessageService
	log *zap.Logger
}

func NewMessageHandler(svc service.MessageService, log *zap.Logger) *MessageHandler {
	return &MessageHandler{svc: svc, log: log}
}

// POST /messages
func (h *MessageHandler) Send(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}
	var req dto.SendMessageRequest
	if !bindJSON(c, &req) {
		return
	}

	msg, err := h.svc.Send(c.Request.Context(), userID, service.SendMessageInput{
		ConversationID:   req.ConversationID,
		RecipientID:      req.RecipientID,
		Content:          req.Content,
		ReplyToMessageID: req.ReplyToMessageID,
	})
	if err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusCreated, msg)
}

// GET /messages?conversation_id=&limit=&before=
func (h *MessageHandler) List(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}
	convID, err := uuid.Parse(c.Query("conversation_id"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}
	var before *uuid.UUID
	if raw := c.Query("before"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			respondError(c, apperror.ErrInvalidUUID, h.log)
			return
		}
		before = &id
	}
	limit := parseLimit(c, 50, 100)

	items, err := h.svc.List(c.Request.Context(), userID, convID, before, limit)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	resp := dto.MessageListResponse{Items: items, Limit: limit}
	if len(items) == limit {
		resp.NextBefore = &items[len(items)-1].ID
	}
	c.JSON(http.StatusOK, resp)
}

// PATCH /messages/:message_id
func (h *MessageHandler) Edit(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}
	messageID, ok := parseUUIDParam(c, "message_id", h.log)
	if !ok {
		return
	}
	var req dto.EditMessageRequest
	if !bindJSON(c, &req) {
		return
	}

	msg, err := h.svc.Edit(c.Request.Context(), userID, messageID, req.Content)
	if err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusOK, msg)
}

// DELETE /messages/:message_id
func (h *MessageHandler) Delete(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}
	messageID, ok := parseUUIDParam(c, "message_id", h.log)
	if !ok {
		return
	}

	if err := h.svc.Delete(c.Request.Context(), userID, messageID); err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusOK, dto.MessageResponse{Success: true, Message: "Сообщение удалено"})
}

// POST /conversations/:id/read
func (h *MessageHandler) MarkRead(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}
	convID, ok := parseUUIDParam(c, "id", h.log)
	if !ok {
		return
	}
	var req dto.MarkReadRequest
	if !bindJSON(c, &req) {
		return
	}

	if err := h.svc.MarkRead(c.Request.Context(), userID, convID, req.MessageID); err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusOK, dto.MessageResponse{Success: true, Message: "Прочитано"})
}

// GET /conversations/:id/messages/:message_id/readers
func (h *MessageHandler) Readers(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}
	convID, ok := parseUUIDParam(c, "id", h.log)
	if !ok {
		return
	}
	messageID, ok := parseUUIDParam(c, "message_id", h.log)
	if !ok {
		return
	}

	ids, err := h.svc.Readers(c.Request.Context(), userID, convID, messageID)
	if err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusOK, dto.ReadersResponse{UserIDs: ids})
}
