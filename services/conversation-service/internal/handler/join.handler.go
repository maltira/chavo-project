package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models/dto"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
)

type JoinHandler struct {
	svc service.JoinService
	log *zap.Logger
}

func NewJoinHandler(svc service.JoinService, log *zap.Logger) *JoinHandler {
	return &JoinHandler{svc: svc, log: log}
}

// GET /invites/:token
func (h *JoinHandler) InviteInfo(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}

	info, err := h.svc.InviteInfo(c.Request.Context(), userID, c.Param("token"))
	if err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusOK, info)
}

// POST /conversations/:id/invite-link/regenerate
func (h *JoinHandler) RegenerateInvite(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}
	convID, ok := parseUUIDParam(c, "id", h.log)
	if !ok {
		return
	}

	token, err := h.svc.RegenerateInvite(c.Request.Context(), userID, convID)
	if err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusOK, dto.InviteLinkResponse{InviteToken: token})
}

// POST /conversations/:id/join-requests
func (h *JoinHandler) RequestJoin(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}
	convID, ok := parseUUIDParam(c, "id", h.log)
	if !ok {
		return
	}
	var req dto.RequestJoinRequest
	if !bindJSON(c, &req) {
		return
	}

	jr, err := h.svc.RequestJoin(c.Request.Context(), userID, convID, req.InviteToken)
	if err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusCreated, jr)
}

// GET /conversations/:id/join-requests?status=&limit=&offset=
func (h *JoinHandler) ListRequests(c *gin.Context) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}
	convID, ok := parseUUIDParam(c, "id", h.log)
	if !ok {
		return
	}
	limit, offset := parseLimit(c, 50, 200), parseOffset(c)

	items, err := h.svc.ListRequests(c.Request.Context(), userID, convID, c.Query("status"), limit, offset)
	if err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusOK, dto.JoinRequestListResponse{Items: items, Limit: limit, Offset: offset})
}

// POST /conversations/:id/join-requests/:request_id/approve
func (h *JoinHandler) Approve(c *gin.Context) {
	h.resolve(c, true)
}

// POST /conversations/:id/join-requests/:request_id/reject
func (h *JoinHandler) Reject(c *gin.Context) {
	h.resolve(c, false)
}

func (h *JoinHandler) resolve(c *gin.Context, approve bool) {
	userID, ok := parseUserID(c, h.log)
	if !ok {
		return
	}
	convID, ok := parseUUIDParam(c, "id", h.log)
	if !ok {
		return
	}
	requestID, ok := parseUUIDParam(c, "request_id", h.log)
	if !ok {
		return
	}

	var err error
	msg := "Заявка отклонена"
	if approve {
		msg = "Заявка одобрена"
		err = h.svc.Approve(c.Request.Context(), userID, convID, requestID)
	} else {
		err = h.svc.Reject(c.Request.Context(), userID, convID, requestID)
	}
	if err != nil {
		respondError(c, err, h.log)
		return
	}
	c.JSON(http.StatusOK, dto.MessageResponse{Success: true, Message: msg})
}
