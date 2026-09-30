package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetUserID извлекает userID из заголовка X-User-ID.
// Возвращает (uuid, true) при успехе или пишет 401 и (Nil, false) при ошибке
func GetXUserID(c *gin.Context) (uuid.UUID, bool) {
	userID, err := uuid.Parse(c.GetHeader("X-User-ID"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":  http.StatusUnauthorized,
			"error": "Необходима авторизация",
		})
		return uuid.Nil, false
	}
	return userID, true
}
