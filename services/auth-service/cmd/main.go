package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/maltira/chavo-project-backend/services/auth-service/internal/config"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/logger"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/router"
	"github.com/maltira/chavo-project-backend/services/auth-service/pkg/db"
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	if err = logger.Init(cfg.Env); err != nil {
		panic(err)
	}
	defer logger.Sync()

	log := logger.Log

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.AUTH_DB_DSN, log)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.ClosePool(pool, log)

	// ЗАПУСК СЕРВЕРА
	r := router.SetupRouter()
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("Auth service starting", zap.String("port", cfg.Port))
		if err = srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Ожидание сигнала завершения
	<-ctx.Done()
	log.Info("Shutting down server...")

	// Graceful shutdown с таймаутом
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err = srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}
	log.Info("Server exited gracefully")
}
