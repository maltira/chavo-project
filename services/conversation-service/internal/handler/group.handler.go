package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models/dto"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
)

type GroupHandler struct {
	svc service.GroupService
	log *zap.Logger
}

func NewGroupHandler(svc service.GroupService, log *zap.Logger) *GroupHandler {
	return &GroupHandler{svc: svc, log: log}
}

// POST /conversations
func (h *GroupHandler) Create(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}
	var req dto.CreateGroupRequest
	if !bindJSON(c, &req) {
		return
	}

	res, err := h.svc.Create(c.Request.Context(), userID, service.CreateGroupInput{
		Name: req.Name, Description: req.Description, AvatarURL: req.AvatarURL,
		Visibility: req.Visibility, MemberIDs: req.MemberIDs,
	})
	if err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusCreated, dto.GroupResponse{Conversation: res.Conversation, InviteToken: res.InviteToken})
}

// PATCH /conversations/:id
func (h *GroupHandler) Update(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}
	convID, ok := parseUUIDParam(c, "id", h.log)
	if !ok {
		return
	}
	var req dto.UpdateGroupRequest
	if !bindJSON(c, &req) {
		return
	}

	res, err := h.svc.Update(c.Request.Context(), userID, convID, service.UpdateGroupInput{
		Name: req.Name, Description: req.Description, AvatarURL: req.AvatarURL, Visibility: req.Visibility,
	})
	if err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusOK, dto.GroupResponse{Conversation: res.Conversation, InviteToken: res.InviteToken})
}

// DELETE /conversations/:id
func (h *GroupHandler) Delete(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}
	convID, ok := parseUUIDParam(c, "id", h.log)
	if !ok {
		return
	}

	if err := h.svc.Delete(c.Request.Context(), userID, convID); err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusOK, dto.MessageResponse{Success: true, Message: "Готово"})
}

// POST /conversations/:id/join
func (h *GroupHandler) Join(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}
	convID, ok := parseUUIDParam(c, "id", h.log)
	if !ok {
		return
	}

	if err := h.svc.Join(c.Request.Context(), userID, convID); err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusOK, dto.MessageResponse{Success: true, Message: "Вы вступили в группу"})
}

// GET /conversations/:id/members?limit=&offset=
func (h *GroupHandler) ListMembers(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}
	convID, ok := parseUUIDParam(c, "id", h.log)
	if !ok {
		return
	}
	limit, offset := parseLimit(c, 50, 200), parseOffset(c)

	items, err := h.svc.ListMembers(c.Request.Context(), userID, convID, limit, offset)
	if err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusOK, dto.MemberListResponse{Items: items, Limit: limit, Offset: offset})
}

// POST /conversations/:id/members
func (h *GroupHandler) AddMembers(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}
	convID, ok := parseUUIDParam(c, "id", h.log)
	if !ok {
		return
	}
	var req dto.AddMembersRequest
	if !bindJSON(c, &req) {
		return
	}

	if err := h.svc.AddMembers(c.Request.Context(), userID, convID, req.UserIDs); err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusOK, dto.MessageResponse{Success: true, Message: "Участники добавлены"})
}

// POST /conversations/:id/members/:user_id
func (h *GroupHandler) SetRole(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}
	convID, ok := parseUUIDParam(c, "id", h.log)
	if !ok {
		return
	}
	targetID, ok := parseUUIDParam(c, "user_id", h.log)
	if !ok {
		return
	}
	var req dto.SetRoleRequest
	if !bindJSON(c, &req) {
		return
	}

	if err := h.svc.SetRole(c.Request.Context(), userID, convID, targetID, req.Role); err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusOK, dto.MessageResponse{Success: true, Message: "Роль обновлена"})
}

// DELETE /conversations/:id/members/:user_id
func (h *GroupHandler) RemoveMember(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}
	convID, ok := parseUUIDParam(c, "id", h.log)
	if !ok {
		return
	}
	targetID, ok := parseUUIDParam(c, "user_id", h.log)
	if !ok {
		return
	}

	if err := h.svc.RemoveMember(c.Request.Context(), userID, convID, targetID); err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusOK, dto.MessageResponse{Success: true, Message: "Участник удалён"})
}

// DELETE /conversations/:id/members/me
func (h *GroupHandler) Leave(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}
	convID, ok := parseUUIDParam(c, "id", h.log)
	if !ok {
		return
	}

	if err := h.svc.RemoveMember(c.Request.Context(), userID, convID, userID); err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusOK, dto.MessageResponse{Success: true, Message: "Вы вышли из группы"})
}
