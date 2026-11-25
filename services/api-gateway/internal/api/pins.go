package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	conversationv1 "github.com/maltira/chavo-project-backend/proto/gen/go/conversation/v1"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/httpx"
)

// pinJSON - закреплённое сообщение; scope "all" - для всех, "me" - только для меня
type pinJSON struct {
	Message  *messageJSON `json:"message"`
	Scope    string       `json:"scope"`
	PinnedBy string       `json:"pinned_by"`
	PinnedAt time.Time    `json:"pinned_at"`
}

func toPin(p *conversationv1.PinnedMessage) pinJSON {
	return pinJSON{
		Message:  toMessage(p.GetMessage()),
		Scope:    p.GetScope(),
		PinnedBy: p.GetPinnedBy(),
		PinnedAt: p.GetPinnedAt().AsTime(),
	}
}

func (h *Handler) listPins(c *gin.Context) {
	resp, err := h.conversations.ListPins(c.Request.Context(), &conversationv1.ListPinsRequest{
		ConversationId: c.Param("id"),
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	items := mapSlice(resp.GetItems(), toPin)
	if items == nil {
		items = []pinJSON{}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) pinMessage(c *gin.Context) {
	var body struct {
		MessageID string `json:"message_id"`
		Scope     string `json:"scope"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	resp, err := h.conversations.PinMessage(c.Request.Context(), &conversationv1.PinMessageRequest{
		ConversationId: c.Param("id"), MessageId: body.MessageID, Scope: body.Scope,
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, toPin(resp.GetPin()))
}

// unpinMessage: ?scope=me - свой личный закреп, иначе общий
func (h *Handler) unpinMessage(c *gin.Context) {
	scope := c.Query("scope")
	if scope == "" {
		scope = "all"
	}
	if _, err := h.conversations.UnpinMessage(c.Request.Context(), &conversationv1.UnpinMessageRequest{
		ConversationId: c.Param("id"), MessageId: c.Param("message_id"), Scope: scope,
	}); err != nil {
		h.fail(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Сообщение откреплено")
}
