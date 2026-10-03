package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maltira/chavo-project-backend/services/conversation-service/config"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/router"
	"github.com/maltira/chavo-project-backend/services/conversation-service/pkg/logger"
	"github.com/maltira/chavo-project-backend/services/conversation-service/pkg/postgres"
	"go.uber.org/zap"
)

type App struct {
	cfg    *config.Config
	log    *zap.Logger
	pool   *pgxpool.Pool
	server *http.Server
}

// New инициализирует инфраструктуру и собирает все зависимости приложения
func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if err := logger.Init(cfg.Env); err != nil {
		return nil, fmt.Errorf("failed to init logger: %w", err)
	}
	log := logger.Log

	ctx := context.Background()

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Repositories

	// Services

	// Handlers

	r := router.SetupRouter()
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	return &App{
		cfg:    cfg,
		log:    log,
		pool:   pool,
		server: server,
	}, nil
}

// Run — точка входа для запуска приложения
func Run() error {
	application, err := New()
	if err != nil {
		return err
	}
	defer application.Close()

	return application.Start()
}

// Start запускает HTTP-сервер
func (a *App) Start() error {
	srvCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		a.log.Info("Conversation service starting", zap.String("port", a.cfg.Port))
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("server failed to start: %w", err)
	case <-srvCtx.Done():
		a.log.Info("Shutting down server...")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		a.log.Error("Server forced to shutdown", zap.Error(err))
		return fmt.Errorf("server forced shutdown: %w", err)
	}

	a.log.Info("Server exited gracefully")
	return nil
}

// Close закрывает все открытые соединения и ресурсы.
func (a *App) Close() {
	if a.server != nil {
		_ = a.server.Close()
	}
	if a.pool != nil {
		postgres.ClosePool(a.pool)
	}
	logger.Sync()
}
