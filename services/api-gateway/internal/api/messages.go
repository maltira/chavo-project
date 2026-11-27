package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	conversationv1 "github.com/maltira/chavo-project-backend/proto/gen/go/conversation/v1"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/httpx"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/linkpreview"
)

const inlinePreviewTimeout = 1500 * time.Millisecond

func (h *Handler) resolveLinkPreview(ctx context.Context, content string, entities []entityJSON, disabled bool) *conversationv1.LinkPreview {
	if disabled || h.previews == nil {
		return nil
	}
	var textLinks []string
	for _, e := range entities {
		if e.Type == "text_link" && e.URL != nil && *e.URL != "" {
			textLinks = append(textLinks, *e.URL)
		}
	}
	target := linkpreview.ExtractFirstURL(content, textLinks)
	if target == "" {
		return nil
	}
	pCtx, cancel := context.WithTimeout(ctx, inlinePreviewTimeout)
	defer cancel()
	p, err := h.previews.Get(pCtx, target)
	if err != nil || p == nil {
		return nil
	}
	return &conversationv1.LinkPreview{
		Url:         p.URL,
		Title:       p.Title,
		Description: p.Description,
		ImageUrl:    p.ImageURL,
		SiteName:    p.SiteName,
	}
}

func (h *Handler) getLinkPreview(c *gin.Context) {
	rawURL := c.Query("url")
	if rawURL == "" {
		httpx.Abort(c, http.StatusBadRequest, "Укажите URL", "")
		return
	}
	if h.previews == nil {
		c.JSON(http.StatusOK, gin.H{"preview": nil})
		return
	}
	p, err := h.previews.Get(c.Request.Context(), rawURL)
	if err != nil {
		if errors.Is(err, linkpreview.ErrInvalidURL) {
			httpx.Abort(c, http.StatusBadRequest, "Некорректная ссылка", "")
			return
		}
		// недоступный или заблокированный хост - просто без превью
		c.JSON(http.StatusOK, gin.H{"preview": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"preview": p})
}

func (h *Handler) listMessages(c *gin.Context) {
	resp, err := h.conversations.ListMessages(c.Request.Context(), &conversationv1.ListMessagesRequest{
		ConversationId: c.Query("conversation_id"),
		Before:         optString(c.Query("before")),
		After:          optString(c.Query("after")),
		Around:         optString(c.Query("around")),
		Limit:          queryInt(c, "limit"),
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	items := make([]*messageJSON, 0, len(resp.GetItems()))
	for _, m := range resp.GetItems() {
		items = append(items, toMessage(m))
	}
	c.JSON(http.StatusOK, gin.H{
		"items":       items,
		"limit":       resp.GetLimit(),
		"next_before": resp.NextBefore,
		"next_after":  resp.NextAfter,
		"has_older":   resp.GetHasOlder(),
		"has_newer":   resp.GetHasNewer(),
	})
}

func (h *Handler) sendMessage(c *gin.Context) {
	var body struct {
		ConversationID     *string      `json:"conversation_id"`
		RecipientID        *string      `json:"recipient_id"`
		Content            string       `json:"content"`
		ReplyToMessageID   *string      `json:"reply_to_message_id"`
		Entities           []entityJSON `json:"entities"`
		IsSilent           bool         `json:"is_silent"`
		DisableLinkPreview bool         `json:"disable_link_preview"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	preview := h.resolveLinkPreview(c.Request.Context(), body.Content, body.Entities, body.DisableLinkPreview)
	resp, err := h.conversations.SendMessage(c.Request.Context(), &conversationv1.SendMessageRequest{
		ConversationId: body.ConversationID, RecipientId: body.RecipientID,
		Content: body.Content, ReplyToMessageId: body.ReplyToMessageID,
		Entities: fromEntities(body.Entities), IsSilent: body.IsSilent, LinkPreview: preview,
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, toMessage(resp.GetMessage()))
}

// forwardMessages: копии сообщений одного чата в мои чаты и/или «Избранное»;
// ответ - созданные копии по чатам
func (h *Handler) forwardMessages(c *gin.Context) {
	var body struct {
		MessageIDs      []string `json:"message_ids"`
		ConversationIDs []string `json:"conversation_ids"`
		ToSaved         bool     `json:"to_saved"`
		HideAuthor      bool     `json:"hide_author"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	resp, err := h.conversations.ForwardMessages(c.Request.Context(), &conversationv1.ForwardMessagesRequest{
		MessageIds: body.MessageIDs, ConversationIds: body.ConversationIDs,
		ToSaved: body.ToSaved, HideAuthor: body.HideAuthor,
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	type forwardedJSON struct {
		ConversationID string         `json:"conversation_id"`
		Messages       []*messageJSON `json:"messages"`
	}
	items := make([]forwardedJSON, len(resp.GetItems()))
	for i, chat := range resp.GetItems() {
		items[i] = forwardedJSON{ConversationID: chat.GetConversationId(), Messages: mapSlice(chat.GetMessages(), toMessage)}
	}
	c.JSON(http.StatusCreated, gin.H{"items": items})
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
		// новое форматирование целиком; нет - без форматирования
		Entities           []entityJSON `json:"entities"`
		DisableLinkPreview bool         `json:"disable_link_preview"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	preview := h.resolveLinkPreview(c.Request.Context(), body.Content, body.Entities, body.DisableLinkPreview)
	resp, err := h.conversations.EditMessage(c.Request.Context(), &conversationv1.EditMessageRequest{
		MessageId: c.Param("message_id"), Content: body.Content, Entities: fromEntities(body.Entities), LinkPreview: preview,
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

func (h *Handler) deleteMessages(c *gin.Context) {
	var body struct {
		MessageIDs []string `json:"message_ids"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	if _, err := h.conversations.DeleteMessages(c.Request.Context(), &conversationv1.DeleteMessagesRequest{
		MessageIds: body.MessageIDs,
	}); err != nil {
		h.fail(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Сообщения удалены")
}

func (h *Handler) addReaction(c *gin.Context) {
	var body struct {
		Emoji string `json:"emoji"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	resp, err := h.conversations.AddReaction(c.Request.Context(), &conversationv1.AddReactionRequest{
		MessageId: c.Param("message_id"),
		Emoji:     body.Emoji,
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"reactions": toReactions(resp.GetReactions())})
}

func (h *Handler) removeReaction(c *gin.Context) {
	emoji := c.Query("emoji")
	if emoji == "" && c.Request.ContentLength != 0 {
		var body struct {
			Emoji string `json:"emoji"`
		}
		if !httpx.BindJSON(c, &body) {
			return
		}
		emoji = body.Emoji
	}
	resp, err := h.conversations.RemoveReaction(c.Request.Context(), &conversationv1.RemoveReactionRequest{
		MessageId: c.Param("message_id"),
		Emoji:     emoji,
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"reactions": toReactions(resp.GetReactions())})
}
