package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models/dto"
)

// respondError сопоставляет ошибку с HTTP-кодом и возвращает JSON-ответ.
// Внутренние ошибки (500) логируются через zap.
func respondError(c *gin.Context, err error, log *zap.Logger) {
	code := apperror.HTTPCode(err)

	if code == http.StatusInternalServerError {
		log.Error("Internal error", zap.Error(err), zap.String("path", c.FullPath()))
	}

	c.JSON(code, dto.ErrorResponse{
		Code:  code,
		Error: apperror.UserMessage(err),
	})
}

// bindJSON парсит body запроса и при ошибке отправляет 400.
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
