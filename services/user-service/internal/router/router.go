package router

import (
	"github.com/gin-gonic/gin"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/handler"
)

func SetupRouter(
	profileH *handler.ProfileHandler,
	blockH *handler.BlockHandler,
	settingsH *handler.SettingsHandler,
) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	users := r.Group("/users")
	{
		users.POST("", profileH.CreateProfile)
		users.GET("/me", profileH.GetMe)
		users.PATCH("/me", profileH.UpdateMe)
		users.GET("", profileH.Search)

		users.GET("/me/blocked", blockH.GetBlocked)
		users.GET("/me/settings", settingsH.GetSettings)
		users.PATCH("/me/settings", settingsH.UpdateSettings)

		users.GET("/:user_id", profileH.GetByID)
		users.GET("/:user_id/block-status", blockH.GetBlockStatus)
		users.POST("/:user_id/block", blockH.BlockUser)
		users.DELETE("/:user_id/block", blockH.UnblockUser)
	}

	return r
}
