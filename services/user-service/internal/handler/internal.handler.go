package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models/dto"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/service"
)

// InternalHandler обслуживает запросы других микросервисов (/internal/*).
// Эти маршруты не должны проксироваться Gateway наружу.
type InternalHandler struct {
	blockSvc    service.BlockService
	profileSvc  service.ProfileService
	settingsSvc service.SettingsService
	log         *zap.Logger
}

func NewInternalHandler(
	blockSvc service.BlockService,
	profileSvc service.ProfileService,
	settingsSvc service.SettingsService,
	log *zap.Logger,
) *InternalHandler {
	return &InternalHandler{blockSvc: blockSvc, profileSvc: profileSvc, settingsSvc: settingsSvc, log: log}
}

// GET /internal/messaging-allowed?sender=&recipient=
func (h *InternalHandler) MessagingAllowed(c *gin.Context) {
	sender, err := uuid.Parse(c.Query("sender"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}
	recipient, err := uuid.Parse(c.Query("recipient"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	blockedBySender, blockedByRecipient, err := h.blockSvc.GetBlockStatus(c.Request.Context(), sender, recipient)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	c.JSON(http.StatusOK, dto.MessagingAllowedResponse{
		Allowed:            !blockedBySender && !blockedByRecipient,
		BlockedBySender:    blockedBySender,
		BlockedByRecipient: blockedByRecipient,
	})
}

// GET /internal/users/:user_id/exists
func (h *InternalHandler) UserExists(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	_, err = h.profileSvc.FindByID(c.Request.Context(), userID)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, dto.UserExistsResponse{Exists: true})
	case errors.Is(err, apperror.ErrNotFound):
		c.JSON(http.StatusOK, dto.UserExistsResponse{Exists: false})
	default:
		respondError(c, err, h.log)
	}
}

// GET /internal/users/:user_id/group-invite-allowed?inviter=
// Приглашение запрещено настройкой пользователя или блокировкой между ним и приглашающим (в любую сторону);
// причина наружу не раскрывается.
func (h *InternalHandler) GroupInviteAllowed(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	inviterID, err := uuid.Parse(c.Query("inviter"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	settings, err := h.settingsSvc.GetSettings(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err, h.log)
		return
	}

	allowed := settings.AllowGroupInvites
	if allowed {
		blockedByInviter, blockedByInvitee, err := h.blockSvc.GetBlockStatus(c.Request.Context(), inviterID, userID)
		if err != nil {
			respondError(c, err, h.log)
			return
		}
		allowed = !blockedByInviter && !blockedByInvitee
	}

	c.JSON(http.StatusOK, dto.GroupInviteAllowedResponse{Allowed: allowed})
}
