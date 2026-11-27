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

	authv1 "github.com/maltira/chavo-project-backend/proto/gen/go/auth/v1"
	"github.com/maltira/chavo-project-backend/services/auth-service/config"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/email"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/grpcserver"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/outbox"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/service"
	"github.com/maltira/chavo-project-backend/services/auth-service/pkg/kafka"
	"github.com/maltira/chavo-project-backend/services/auth-service/pkg/logger"
	"github.com/maltira/chavo-project-backend/services/auth-service/pkg/postgres"
	rds "github.com/maltira/chavo-project-backend/services/auth-service/pkg/redis"
)

type App struct {
	cfg       *config.Config
	log       *zap.Logger
	pool      *pgxpool.Pool
	rdb       *redis.Client
	producer  *kafka.Producer
	publisher *outbox.Publisher
	server    *grpc.Server
	health    *health.Server
}

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

	userRepo := repository.NewUserRepository(pool)
	verRepo := repository.NewVerificationRepository(pool)
	resetRepo := repository.NewPasswordResetRepository(pool)
	tokenRepo := repository.NewTokenRepository(pool)
	backupRepo := repository.NewBackupCodeRepository(pool)
	outboxRepo := repository.NewOutboxRepository(pool)
	publisher := outbox.NewPublisher(pool, outboxRepo, producer, log, outbox.DefaultOptions())

	otpSvc := service.NewOtpService(rdb, userRepo, backupRepo, cfg.TOTPEncryptionKey, mail, log, cfg.TOTPEncryptionPrevKeys...)
	tokenSvc := service.NewTokenService(tokenRepo, outboxRepo, pool, rdb, publisher.Notify, cfg, log)
	authSvc := service.NewAuthService(userRepo, verRepo, resetRepo, tokenRepo, backupRepo, outboxRepo, otpSvc, pool, rdb, publisher.Notify, mail, cfg, log)

	server := grpc.NewServer(grpc.UnaryInterceptor(grpcserver.UnaryInterceptor(log)))
	authv1.RegisterAuthServiceServer(server, grpcserver.New(authSvc, otpSvc, tokenSvc))

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(server, healthSrv)
	if cfg.Env != "production" {
		reflection.Register(server)
	}

	return &App{
		cfg:       cfg,
		log:       log,
		pool:      pool,
		rdb:       rdb,
		producer:  producer,
		publisher: publisher,
		server:    server,
		health:    healthSrv,
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

// Start запускает gRPC-сервер и outbox publisher до SIGINT/SIGTERM.
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
		a.publisher.Run(ctx)
	}()

	serverErr := make(chan error, 1)
	go func() {
		a.log.Info("Auth service (gRPC) starting", zap.String("port", a.cfg.Port))
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

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		a.log.Error("Outbox publisher did not stop in time")
	}

	a.log.Info("Server exited gracefully")
	return nil
}

func (a *App) Close() {
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
