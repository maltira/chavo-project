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

	"github.com/maltira/chavo-project-backend/services/user-service/config"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/handler"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/router"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/service"
	"github.com/maltira/chavo-project-backend/services/user-service/pkg/db"
	pkgkafka "github.com/maltira/chavo-project-backend/services/user-service/pkg/kafka"
	"github.com/maltira/chavo-project-backend/services/user-service/pkg/logger"
	appredis "github.com/maltira/chavo-project-backend/services/user-service/pkg/redis"
)

type App struct {
	cfg      *config.Config
	log      *zap.Logger
	pool     *pgxpool.Pool
	rdb      *redis.Client
	producer *pkgkafka.Producer
	server   *http.Server
}

func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if err = logger.Init(cfg.Env); err != nil {
		return nil, fmt.Errorf("failed to init logger: %w", err)
	}
	log := logger.Log

	ctx := context.Background()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL, log)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	rdb, err := appredis.NewClient(ctx, cfg.RedisURL, log)
	if err != nil {
		db.ClosePool(pool, log)
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	producer := pkgkafka.NewProducer(cfg.KafkaBrokers, log)

	pRepo := repository.NewProfileRepository(pool)
	bRepo := repository.NewBlockRepository(pool)
	sRepo := repository.NewSettingsRepository(pool)

	profileSvc := service.NewProfileService(pRepo)
	blockSvc := service.NewBlockService(bRepo, producer, log)
	settingsSvc := service.NewSettingsService(sRepo)

	profileH := handler.NewProfileHandler(profileSvc, rdb, log)
	blockH := handler.NewBlockHandler(blockSvc, log)
	settingsH := handler.NewSettingsHandler(settingsSvc, log)

	r := router.SetupRouter(profileH, blockH, settingsH)
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

func Run() error {
	application, err := New()
	if err != nil {
		return err
	}
	defer application.Close()
	return application.Start()
}

func (a *App) Start() error {
	srvCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		a.log.Info("User service starting", zap.String("port", a.cfg.Port))
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

func (a *App) Close() {
	if a.server != nil {
		_ = a.server.Close()
	}
	if a.producer != nil {
		_ = a.producer.Close()
	}
	if a.pool != nil {
		db.ClosePool(a.pool, a.log)
	}
	if a.rdb != nil {
		appredis.Close(a.rdb, a.log)
	}
	logger.Sync()
}
