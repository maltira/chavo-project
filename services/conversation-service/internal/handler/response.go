package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models/dto"
)

// respondError сопоставляет ошибку с HTTP-кодом; внутренние ошибки (500) логируются.
func respondError(c *gin.Context, err error, log *zap.Logger) {
	code := apperror.HTTPCode(err)

	if code == http.StatusInternalServerError {
		log.Error("Internal error", zap.Error(err), zap.String("path", c.FullPath()))
	}

	c.JSON(code, dto.ErrorResponse{
		Code:   code,
		Error:  apperror.UserMessage(err),
		Reason: apperror.Reason(err),
	})
}

func bindJSON(c *gin.Context, obj any) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:  http.StatusBadRequest,
			Error: apperror.UserMessage(apperror.ErrIncorrectData),
		})
		return false
	}
	return true
}

// parseUserID читает идентификатор пользователя из X-User-ID, проставленного Gateway.
func parseUserID(c *gin.Context, log *zap.Logger) (uuid.UUID, bool) {
	userID, err := uuid.Parse(c.GetHeader("X-User-ID"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, log)
		return uuid.Nil, false
	}
	return userID, true
}

func parseUUIDParam(c *gin.Context, name string, log *zap.Logger) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, log)
		return uuid.Nil, false
	}
	return id, true
}
