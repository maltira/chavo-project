package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/auth-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/models/dto"
)

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
