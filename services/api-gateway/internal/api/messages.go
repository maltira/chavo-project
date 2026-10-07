package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	conversationv1 "github.com/maltira/chavo-project-backend/proto/gen/go/conversation/v1"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/httpx"
)

func (h *Handler) listMessages(c *gin.Context) {
	resp, err := h.conversations.ListMessages(c.Request.Context(), &conversationv1.ListMessagesRequest{
		ConversationId: c.Query("conversation_id"), Before: optString(c.Query("before")), Limit: queryInt(c, "limit"),
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	items := make([]*messageJSON, 0, len(resp.GetItems()))
	for _, m := range resp.GetItems() {
		items = append(items, toMessage(m))
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "limit": resp.GetLimit(), "next_before": resp.NextBefore})
}

func (h *Handler) sendMessage(c *gin.Context) {
	var body struct {
		ConversationID   *string `json:"conversation_id"`
		RecipientID      *string `json:"recipient_id"`
		Content          string  `json:"content"`
		ReplyToMessageID *string `json:"reply_to_message_id"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	resp, err := h.conversations.SendMessage(c.Request.Context(), &conversationv1.SendMessageRequest{
		ConversationId: body.ConversationID, RecipientId: body.RecipientID,
		Content: body.Content, ReplyToMessageId: body.ReplyToMessageID,
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, toMessage(resp.GetMessage()))
}

func (h *Handler) getMessage(c *gin.Context) {
	resp, err := h.conversations.GetMessage(c.Request.Context(), &conversationv1.GetMessageRequest{MessageId: c.Param("message_id")})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toMessage(resp.GetMessage()))
}

func (h *Handler) editMessage(c *gin.Context) {
	var body struct {
		Content string `json:"content"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	resp, err := h.conversations.EditMessage(c.Request.Context(), &conversationv1.EditMessageRequest{
		MessageId: c.Param("message_id"), Content: body.Content,
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toMessage(resp.GetMessage()))
}

func (h *Handler) deleteMessage(c *gin.Context) {
	if _, err := h.conversations.DeleteMessage(c.Request.Context(), &conversationv1.DeleteMessageRequest{
		MessageId: c.Param("message_id"),
	}); err != nil {
		h.fail(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Сообщение удалено")
}
