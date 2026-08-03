package middleware

import (
	"net/http"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models/dto"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func ValidateUUID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		if _, err := uuid.Parse(id); err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, dto.ErrorResponse{Code: 400, Error: "invalid UUID"})
			return
		}
		c.Next()
	}
}
