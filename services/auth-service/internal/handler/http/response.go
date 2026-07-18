package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/auth-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/dto"
)

// respondError maps an apperror to the proper HTTP status and writes a JSON error response.
// Internal errors are logged but never exposed to the client.
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

// bindJSON parses the request body and responds with 400 on failure.
// Returns true if binding succeeded, false otherwise.
func bindJSON(c *gin.Context, obj any) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:  http.StatusBadRequest,
			Error: "Некорректные входные данные",
		})
		return false
	}
	return true
}
