package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/maltira/chavo-project-backend/services/auth-service/internal/logger"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/router"
	"github.com/maltira/chavo-project-backend/services/auth-service/pkg/db"
	"go.uber.org/zap"
)

func main() {
	ctx := context.Background()
	if err := logger.Init(); err != nil {
		panic(err)
	}
	defer logger.Sync()

	log := logger.Log

	pool, err := db.NewPool(ctx, log)
	if err != nil {
		logger.Log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.ClosePool(pool, log)

	// ЗАПУСК СЕРВЕРА
	r := router.SetupRouter()
	port := os.Getenv("AUTH_PORT")
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		logger.Log.Info("Auth service starting", zap.String("port", port))
		if err = srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Ожидание сигнала завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Log.Info("Shutting down server...")

	// Graceful shutdown с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err = srv.Shutdown(ctx); err != nil {
		logger.Log.Error("Server forced to shutdown", zap.Error(err))
	}
	logger.Log.Info("Server exited gracefully")
}
