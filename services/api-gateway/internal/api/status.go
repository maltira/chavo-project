package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	conversationv1 "github.com/maltira/chavo-project-backend/proto/gen/go/conversation/v1"
	statusv1 "github.com/maltira/chavo-project-backend/proto/gen/go/status/v1"
	userv1 "github.com/maltira/chavo-project-backend/proto/gen/go/user/v1"
	"github.com/maltira/chavo-project-backend/proto/status"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/httpx"
)

// statusJSON — статус в ответах API и событиях: иконка из набора, текст, цвета пользователя, срок.
type statusJSON struct {
	Icon       string     `json:"icon"`
	Text       string     `json:"text"`
	Background string     `json:"background"`
	Foreground string     `json:"foreground"`
	ExpiresAt  *time.Time `json:"expires_at"`
}

// toStatus: null, если статуса нет или его иконку убрали из набора.
func toStatus(s *statusv1.Status) *statusJSON {
	if s == nil || !status.KnownIcon(s.GetIcon()) {
		return nil
	}
	return &statusJSON{
		Icon: s.GetIcon(), Text: s.GetText(), Background: s.GetBackground(), Foreground: s.GetForeground(),
		ExpiresAt: tsPtr(s.GetExpiresAt()),
	}
}

type statusCatalogJSON struct {
	Icons     []string `json:"icons"`
	Durations []string `json:"durations"`
}

// listStatuses — GET /api/statuses: набор иконок и сроков отдаётся как есть, без вызова сервисов.
func (h *Handler) listStatuses(c *gin.Context) {
	durations := status.Durations()
	keys := make([]string, len(durations))
	for i, d := range durations {
		keys[i] = d.Key
	}
	c.Header("Cache-Control", "public, max-age=3600")
	c.JSON(http.StatusOK, statusCatalogJSON{Icons: status.Icons(), Durations: keys})
}

// setStatusBody: все поля обязательны; цвета — #RRGGBB.
type setStatusBody struct {
	Icon       string `json:"icon"`
	Text       string `json:"text"`
	Background string `json:"background"`
	Foreground string `json:"foreground"`
	Duration   string `json:"duration"`
}

// setMyStatus — POST /api/users/me/status: каждый вызов ставит новый статус (срок от момента запроса),
// прежний заменяется целиком.
func (h *Handler) setMyStatus(c *gin.Context) {
	var body setStatusBody
	if !httpx.BindJSON(c, &body) {
		return
	}
	resp, err := h.users.SetStatus(c.Request.Context(), &userv1.SetStatusRequest{
		Icon: body.Icon, Text: body.Text, Background: body.Background, Foreground: body.Foreground, Duration: body.Duration,
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, toStatus(resp.GetStatus()))
}

func (h *Handler) clearMyStatus(c *gin.Context) {
	if _, err := h.users.ClearStatus(c.Request.Context(), &userv1.ClearStatusRequest{}); err != nil {
		h.fail(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Статус удалён")
}

func (h *Handler) setGroupStatus(c *gin.Context) {
	var body setStatusBody
	if !httpx.BindJSON(c, &body) {
		return
	}
	resp, err := h.conversations.SetGroupStatus(c.Request.Context(), &conversationv1.SetGroupStatusRequest{
		ConversationId: c.Param("id"), Icon: body.Icon, Text: body.Text,
		Background: body.Background, Foreground: body.Foreground, Duration: body.Duration,
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, toStatus(resp.GetStatus()))
}

func (h *Handler) clearGroupStatus(c *gin.Context) {
	if _, err := h.conversations.ClearGroupStatus(c.Request.Context(), &conversationv1.ClearGroupStatusRequest{
		ConversationId: c.Param("id"),
	}); err != nil {
		h.fail(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Статус группы удалён")
}
