package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/maltira/chavo-project-backend/proto/avatarshape"
	avatarshapev1 "github.com/maltira/chavo-project-backend/proto/gen/go/avatarshape/v1"
	conversationv1 "github.com/maltira/chavo-project-backend/proto/gen/go/conversation/v1"
	userv1 "github.com/maltira/chavo-project-backend/proto/gen/go/user/v1"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/httpx"
)

// avatarShapeJSON - фигура позади аватара: ключ из каталога и цвет #RRGGBB
type avatarShapeJSON struct {
	Shape string `json:"shape"`
	Color string `json:"color"`
}

// toAvatarShape - null, если фигуры нет или её убрали из каталога
func toAvatarShape(s *avatarshapev1.AvatarShape) *avatarShapeJSON {
	if s == nil || !avatarshape.Known(s.GetShape()) {
		return nil
	}
	return &avatarShapeJSON{Shape: s.GetShape(), Color: s.GetColor()}
}

// setAvatarShapeBody - оба поля обязательны
type setAvatarShapeBody struct {
	Shape string `json:"shape"`
	Color string `json:"color"`
}

// setMyAvatarShape - POST /api/users/me/avatar-shape: новая фигура заменяет прежнюю целиком
func (h *Handler) setMyAvatarShape(c *gin.Context) {
	var body setAvatarShapeBody
	if !httpx.BindJSON(c, &body) {
		return
	}
	resp, err := h.users.SetAvatarShape(c.Request.Context(), &userv1.SetAvatarShapeRequest{Shape: body.Shape, Color: body.Color})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, toAvatarShape(resp.GetAvatarShape()))
}

func (h *Handler) clearMyAvatarShape(c *gin.Context) {
	if _, err := h.users.ClearAvatarShape(c.Request.Context(), &userv1.ClearAvatarShapeRequest{}); err != nil {
		h.fail(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Фигура удалена")
}

func (h *Handler) setGroupAvatarShape(c *gin.Context) {
	var body setAvatarShapeBody
	if !httpx.BindJSON(c, &body) {
		return
	}
	resp, err := h.conversations.SetGroupAvatarShape(c.Request.Context(), &conversationv1.SetGroupAvatarShapeRequest{
		ConversationId: c.Param("id"), Shape: body.Shape, Color: body.Color,
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, toAvatarShape(resp.GetAvatarShape()))
}

func (h *Handler) clearGroupAvatarShape(c *gin.Context) {
	if _, err := h.conversations.ClearGroupAvatarShape(c.Request.Context(), &conversationv1.ClearGroupAvatarShapeRequest{
		ConversationId: c.Param("id"),
	}); err != nil {
		h.fail(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Фигура группы удалена")
}
