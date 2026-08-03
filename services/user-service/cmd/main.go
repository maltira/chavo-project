package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/user-service/config"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/router"
	"github.com/maltira/chavo-project-backend/services/user-service/pkg/db"
	"github.com/maltira/chavo-project-backend/services/user-service/pkg/logger"
	appredis "github.com/maltira/chavo-project-backend/services/user-service/pkg/redis"
)

func main() {
	cfg, pool, rdb, cleanup := initInfra()
	defer cleanup()

	log := logger.Log
	srv := buildApp(cfg, pool, rdb, log)
	runServer(srv, cfg.UserPort, log)
}

// initInfra загружает конфиг, инициализирует логгер, PostgreSQL (pgxpool) и Redis.
func initInfra() (cfg *config.Config, pool *pgxpool.Pool, rdb *redis.Client, cleanup func()) {
	var err error

	cfg, err = config.Load()
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	if err = logger.Init(cfg.Env); err != nil {
		panic(err)
	}
	log := logger.Log
	ctx := context.Background()

	pool, err = db.NewPool(ctx, cfg.UserDBDSN, log)
	if err != nil {
		log.Fatal("Failed to connect to database pool", zap.Error(err))
	}

	rdb, err = appredis.NewClient(ctx, cfg.RedisURL, log)
	if err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	cleanup = func() {
		db.ClosePool(pool, log)
		appredis.Close(rdb, log)
		logger.Sync()
	}

	return cfg, pool, rdb, cleanup
}

// buildApp собирает приложение: репозитории → сервисы → хендлеры → роутер.
func buildApp(cfg *config.Config, pool *pgxpool.Pool, rdb *redis.Client, log *zap.Logger) *http.Server {
	r := router.SetupRouter(pool, rdb, cfg.InternalSecret, log)
	return &http.Server{
		Addr:    ":" + cfg.UserPort,
		Handler: r,
	}
}

// runServer запускает HTTP-сервер и обрабатывает graceful shutdown.
func runServer(srv *http.Server, port string, log *zap.Logger) {
	srvCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("User service starting", zap.String("port", port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	<-srvCtx.Done()
	log.Info("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}
	log.Info("Server exited gracefully")
}
