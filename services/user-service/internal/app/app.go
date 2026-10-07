package app

import (
	"context"
	"fmt"
	"net"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	userv1 "github.com/maltira/chavo-project-backend/proto/gen/go/user/v1"
	"github.com/maltira/chavo-project-backend/services/user-service/config"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/grpcserver"
	consumer "github.com/maltira/chavo-project-backend/services/user-service/internal/kafka"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/presence"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/repository"
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
	consumer *consumer.PresenceConsumer
	server   *grpc.Server
	health   *health.Server
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

	profileSvc := service.NewProfileService(repository.NewProfileRepository(pool))
	blockSvc := service.NewBlockService(repository.NewBlockRepository(pool), producer, log)
	settingsSvc := service.NewSettingsService(repository.NewSettingsRepository(pool))

	srv := grpcserver.New(profileSvc, blockSvc, settingsSvc, presence.NewChecker(rdb))
	server := grpc.NewServer(grpc.UnaryInterceptor(grpcserver.UnaryInterceptor(log)))
	userv1.RegisterUserServiceServer(server, srv)
	userv1.RegisterUserInternalServiceServer(server, srv)

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(server, healthSrv)
	if cfg.Env != "production" {
		reflection.Register(server)
	}

	return &App{
		cfg:      cfg,
		log:      log,
		pool:     pool,
		rdb:      rdb,
		producer: producer,
		consumer: consumer.NewPresenceConsumer(cfg.KafkaBrokers, profileSvc, log),
		server:   server,
		health:   healthSrv,
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

// Start запускает gRPC-сервер и consumer presence-events до SIGINT/SIGTERM.
func (a *App) Start() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	lis, err := net.Listen("tcp", ":"+a.cfg.Port)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		a.consumer.Run(ctx)
	}()

	serverErr := make(chan error, 1)
	go func() {
		a.log.Info("User service (gRPC) starting", zap.String("port", a.cfg.Port))
		serverErr <- a.server.Serve(lis)
	}()
	a.health.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	select {
	case err := <-serverErr:
		stop()
		wg.Wait()
		return fmt.Errorf("grpc server failed: %w", err)
	case <-ctx.Done():
		a.log.Info("Shutting down server...")
	}

	a.health.Shutdown()
	stopped := make(chan struct{})
	go func() {
		a.server.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		a.log.Error("Server forced to shutdown")
		a.server.Stop()
	}
	wg.Wait()

	a.log.Info("Server exited gracefully")
	return nil
}

func (a *App) Close() {
	if a.consumer != nil {
		_ = a.consumer.Close()
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
