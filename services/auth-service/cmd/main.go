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

	"github.com/maltira/chavo-project-backend/services/auth-service/config"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/email"
	handler "github.com/maltira/chavo-project-backend/services/auth-service/internal/handler/http"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/logger"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/router"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/service"
	"github.com/maltira/chavo-project-backend/services/auth-service/pkg/db"
	appredis "github.com/maltira/chavo-project-backend/services/auth-service/pkg/redis"
)

func main() {
	cfg, pool, rdb, cleanup := initInfra()
	defer cleanup()

	log := logger.Log
	srv := buildApp(cfg, pool, rdb, log)
	runServer(srv, cfg.AuthPort, log)
}

// initInfra загружает конфиг, инициализирует логгер, БД и Redis.
// Возвращает cleanup-функцию, которую нужно вызвать через defer.
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

	pool, err = db.NewPool(ctx, cfg.AuthDBDSN, log)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
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

// buildApp собирает слои приложения: репозитории → сервисы → хендлеры → роутер.
func buildApp(cfg *config.Config, pool *pgxpool.Pool, rdb *redis.Client, log *zap.Logger) *http.Server {
	mail := email.NewSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, log)

	// Repositories
	userRepo := repository.NewUserRepository(pool)
	tokenRepo := repository.NewTokenRepository(pool)
	otpRepo := repository.NewOTPRepository(pool)

	// Services
	tokenSvc := service.NewTokenService(tokenRepo, rdb, cfg, log)
	otpSvc := service.NewOtpService(otpRepo, mail, log)
	authSvc := service.NewAuthService(userRepo, tokenRepo, pool, rdb, mail, cfg, log)

	// Handlers
	authHandler := handler.NewAuthHandler(authSvc, otpSvc, tokenSvc, log)
	otpHandler := handler.NewOtpHandler(otpSvc, authSvc, tokenSvc, cfg, log)
	refreshHandler := handler.NewRefreshHandler(tokenSvc, cfg, log)

	r := router.SetupRouter(authHandler, otpHandler, refreshHandler)
	return &http.Server{
		Addr:    ":" + cfg.AuthPort,
		Handler: r,
	}
}

// runServer запускает HTTP-сервер и ждёт сигнала завершения (SIGINT/SIGTERM),
// после чего выполняет graceful shutdown с таймаутом 10 секунд.
func runServer(srv *http.Server, port string, log *zap.Logger) {
	srvCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("Auth service starting", zap.String("port", port))
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

