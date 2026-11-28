package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	conversationv1 "github.com/maltira/chavo-project-backend/proto/gen/go/conversation/v1"
	profilecoverv1 "github.com/maltira/chavo-project-backend/proto/gen/go/profilecover/v1"
	userv1 "github.com/maltira/chavo-project-backend/proto/gen/go/user/v1"
	"github.com/maltira/chavo-project-backend/proto/profilecover"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/httpx"
)

// profileCoverJSON - обложка шапки профиля или группы: цвет #RRGGBB и необязательный паттерн
type profileCoverJSON struct {
	Color   string `json:"color"`
	Pattern string `json:"pattern"`
}

// toProfileCover - null, если обложки нет; неизвестный паттерн деградирует в пустую строку
func toProfileCover(c *profilecoverv1.ProfileCover) *profileCoverJSON {
	if c == nil || c.GetColor() == "" {
		return nil
	}
	pattern := c.GetPattern()
	if pattern != "" && !profilecover.Known(pattern) {
		pattern = ""
	}
	return &profileCoverJSON{Color: c.GetColor(), Pattern: pattern}
}

// setProfileCoverBody - цвет обязателен, паттерн может быть пустым (только градиент)
type setProfileCoverBody struct {
	Color   string `json:"color"`
	Pattern string `json:"pattern"`
}

// setMyProfileCover - POST /api/users/me/profile-cover: новая обложка заменяет прежнюю целиком
func (h *Handler) setMyProfileCover(c *gin.Context) {
	var body setProfileCoverBody
	if !httpx.BindJSON(c, &body) {
		return
	}
	resp, err := h.users.SetProfileCover(c.Request.Context(), &userv1.SetProfileCoverRequest{
		Color: body.Color, Pattern: body.Pattern,
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, toProfileCover(resp.GetProfileCover()))
}

func (h *Handler) clearMyProfileCover(c *gin.Context) {
	if _, err := h.users.ClearProfileCover(c.Request.Context(), &userv1.ClearProfileCoverRequest{}); err != nil {
		h.fail(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Обложка удалена")
}

func (h *Handler) setGroupProfileCover(c *gin.Context) {
	var body setProfileCoverBody
	if !httpx.BindJSON(c, &body) {
		return
	}
	resp, err := h.conversations.SetGroupProfileCover(c.Request.Context(), &conversationv1.SetGroupProfileCoverRequest{
		ConversationId: c.Param("id"), Color: body.Color, Pattern: body.Pattern,
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, toProfileCover(resp.GetProfileCover()))
}

func (h *Handler) clearGroupProfileCover(c *gin.Context) {
	if _, err := h.conversations.ClearGroupProfileCover(c.Request.Context(), &conversationv1.ClearGroupProfileCoverRequest{
		ConversationId: c.Param("id"),
	}); err != nil {
		h.fail(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Обложка группы удалена")
}
