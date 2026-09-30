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
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/auth-service/config"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/email"
	handler "github.com/maltira/chavo-project-backend/services/auth-service/internal/handler/http"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/router"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/service"
	"github.com/maltira/chavo-project-backend/services/auth-service/pkg/kafka"
	"github.com/maltira/chavo-project-backend/services/auth-service/pkg/logger"
	"github.com/maltira/chavo-project-backend/services/auth-service/pkg/postgres"
	rds "github.com/maltira/chavo-project-backend/services/auth-service/pkg/redis"
)

type App struct {
	cfg      *config.Config
	log      *zap.Logger
	pool     *pgxpool.Pool
	rdb      *redis.Client
	producer *kafka.Producer
	server   *http.Server
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

	rdb, err := rds.NewClient(ctx, cfg.RedisURL)
	if err != nil {
		postgres.ClosePool(pool)
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	producer := kafka.NewProducer(cfg.KafkaBrokers, log)

	mail := email.NewSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, log)

	// Repositories
	userRepo := repository.NewUserRepository(pool)
	verRepo := repository.NewVerificationRepository(pool)
	resetRepo := repository.NewPasswordResetRepository(pool)
	tokenRepo := repository.NewTokenRepository(pool)

	// Services
	otpSvc := service.NewOtpService(rdb, userRepo, mail, log)
	tokenSvc := service.NewTokenService(tokenRepo, rdb, producer, cfg, log)
	authSvc := service.NewAuthService(userRepo, verRepo, resetRepo, tokenRepo, otpSvc, pool, rdb, producer, mail, cfg, log)

	// Handlers
	authHandler := handler.NewAuthHandler(authSvc, tokenSvc, log)
	otpHandler := handler.NewOtpHandler(otpSvc, tokenSvc, cfg, log)
	refreshHandler := handler.NewRefreshHandler(tokenSvc, cfg, log)

	r := router.SetupRouter(authHandler, otpHandler, refreshHandler)
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	return &App{
		cfg:      cfg,
		log:      log,
		pool:     pool,
		rdb:      rdb,
		producer: producer,
		server:   server,
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
		a.log.Info("Auth service starting", zap.String("port", a.cfg.Port))
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
	if a.producer != nil {
		_ = a.producer.Close()
	}
	if a.pool != nil {
		postgres.ClosePool(a.pool)
	}
	if a.rdb != nil {
		rds.Close(a.rdb)
	}
	logger.Sync()
}
