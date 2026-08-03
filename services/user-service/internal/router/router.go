package router

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/handler"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/middleware"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/service"
	"github.com/maltira/chavo-project-backend/services/user-service/pkg/utils"
)

// SetupRouter собирает все зависимости и возвращает настроенный Gin-роутер.
func SetupRouter(
	pool *pgxpool.Pool,
	rdb *redis.Client,
	internalSecret string,
	log *zap.Logger,
) *gin.Engine {
	// Repositories
	pRepo := repository.NewProfileRepository(pool)
	bRepo := repository.NewBlockRepository(pool)
	sRepo := repository.NewSettingsRepository(pool)

	// Services
	pSvc := service.NewProfileService(pRepo)
	bSvc := service.NewBlockService(bRepo, pRepo)
	sSvc := service.NewSettingsService(sRepo)

	// StatusManager с UpdateLastSeen-колбэком из репозитория
	statusMgr := utils.NewStatusManager(rdb, log, pRepo.UpdateLastSeen)

	// Handlers
	profileH := handler.NewProfileHandler(pSvc, log)
	blockH := handler.NewBlockHandler(bSvc, rdb, log)
	settingsH := handler.NewSettingsHandler(sSvc, log)
	wsH := handler.NewWSHandler(rdb, statusMgr, log)

	// Запускаем Redis Pub/Sub-подписки в фоне
	ctx := context.Background()
	go wsH.PubSubBlock(ctx)
	go wsH.PubSubStatus(ctx)
	go wsH.PubSubNewMessage(ctx)
	go wsH.PubSubReadAck(ctx)

	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api/user")

	initProfileRoutes(api, profileH, internalSecret)
	initBlockRoutes(api, blockH)
	initSettingsRoutes(api, settingsH)
	initWebSocketRoutes(api, wsH)

	return r
}

func initProfileRoutes(api *gin.RouterGroup, h *handler.ProfileHandler, internalSecret string) {
	g := api.Group("/profile")
	{
		g.GET("/search", h.GetProfilesByQuery)
		g.GET("", h.GetCurrentProfile)
		g.PUT("", h.UpdateProfile)
		g.POST("", middleware.InternalOnly(internalSecret), h.CreateProfile)
		g.GET("/:id", middleware.ValidateUUID(), h.GetProfileByID)
		g.GET("/check-username", h.IsUsernameFree)
		g.GET("/check-status/:id", middleware.ValidateUUID(), h.GetUserStatus)
	}
}

func initBlockRoutes(api *gin.RouterGroup, h *handler.BlockHandler) {
	g := api.Group("/block")
	{
		g.GET("/all", h.GetAllBlocks)
		g.POST("/:id", middleware.ValidateUUID(), h.BlockUser)
		g.DELETE("/:id", middleware.ValidateUUID(), h.UnblockUser)
		g.GET("/check/:id", middleware.ValidateUUID(), h.IsBlocked)
	}
}

func initSettingsRoutes(api *gin.RouterGroup, h *handler.SettingsHandler) {
	g := api.Group("/settings")
	{
		g.GET("", h.GetSettings)
		g.PUT("/status", h.UpdateVisibleStatus)
		g.PUT("/birth", h.UpdateVisibleBirthDate)
	}
}

func initWebSocketRoutes(api *gin.RouterGroup, h *handler.WSHandler) {
	api.GET("/ws", h.Connect)
}
