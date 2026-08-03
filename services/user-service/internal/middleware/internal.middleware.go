package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/models/dto"
)

// InternalOnly проверяет X-Internal-Secret заголовок для inter-service вызовов.
// Secret передаётся явно через параметр (не через os.Getenv напрямую).
func InternalOnly(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if secret == "" || c.GetHeader("X-Internal-Secret") != secret {
			c.AbortWithStatusJSON(http.StatusForbidden, dto.ErrorResponse{Code: 403, Error: "forbidden"})
			return
		}
		c.Next()
	}
}
