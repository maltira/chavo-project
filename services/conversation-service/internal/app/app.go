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
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	conversationv1 "github.com/maltira/chavo-project-backend/proto/gen/go/conversation/v1"
	"github.com/maltira/chavo-project-backend/proto/grpcx"
	"github.com/maltira/chavo-project-backend/services/conversation-service/config"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/client/userclient"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/grpcserver"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/outbox"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
	"github.com/maltira/chavo-project-backend/services/conversation-service/pkg/crypto"
	pkgkafka "github.com/maltira/chavo-project-backend/services/conversation-service/pkg/kafka"
	"github.com/maltira/chavo-project-backend/services/conversation-service/pkg/logger"
	"github.com/maltira/chavo-project-backend/services/conversation-service/pkg/postgres"
)

const healthCheckInterval = 10 * time.Second

type App struct {
	cfg       *config.Config
	log       *zap.Logger
	pool      *pgxpool.Pool
	userConn  *grpc.ClientConn
	producer  *pkgkafka.Producer
	publisher *outbox.Publisher
	server    *grpc.Server
	health    *health.Server
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

	pool, err := postgres.NewPool(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	cipher, err := crypto.NewCipher(cfg.EncryptionKey)
	if err != nil {
		postgres.ClosePool(pool)
		return nil, fmt.Errorf("failed to init cipher: %w", err)
	}

	userConn, err := grpc.NewClient(cfg.UserServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(grpcx.PropagateTrace()))
	if err != nil {
		postgres.ClosePool(pool)
		return nil, fmt.Errorf("failed to create user-service client: %w", err)
	}

	db := repository.NewDB(pool)
	convRepo := repository.NewConversationRepository()
	msgRepo := repository.NewMessageRepository()
	outboxRepo := repository.NewOutboxRepository()
	joinRepo := repository.NewJoinRequestRepository()
	banRepo := repository.NewBanRepository()
	users := userclient.New(userConn)
	producer := pkgkafka.NewProducer(cfg.KafkaBrokers, log)
	publisher := outbox.NewPublisher(db, outboxRepo, producer, cipher, log, outbox.DefaultOptions())

	srv := grpcserver.New(
		service.NewConversationService(db, convRepo, cipher),
		service.NewGroupService(db, convRepo, outboxRepo, users, banRepo),
		service.NewJoinService(db, convRepo, joinRepo, outboxRepo, banRepo),
		service.NewMessageService(db, convRepo, msgRepo, outboxRepo, users, cipher, log),
	)
	server := grpc.NewServer(grpc.UnaryInterceptor(grpcserver.UnaryInterceptor(log)))
	conversationv1.RegisterConversationServiceServer(server, srv)
	conversationv1.RegisterConversationInternalServiceServer(server, srv)

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(server, healthSrv)
	if cfg.Env != "production" {
		reflection.Register(server)
	}

	return &App{
		cfg:       cfg,
		log:       log,
		pool:      pool,
		userConn:  userConn,
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

// Start запускает gRPC-сервер, outbox publisher и проверку БД для health до SIGINT/SIGTERM.
func (a *App) Start() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	lis, err := net.Listen("tcp", ":"+a.cfg.Port)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		a.publisher.Run(ctx)
	}()
	go func() {
		defer wg.Done()
		a.watchDatabase(ctx)
	}()

	serverErr := make(chan error, 1)
	go func() {
		a.log.Info("Conversation service (gRPC) starting", zap.String("port", a.cfg.Port))
		serverErr <- a.server.Serve(lis)
	}()

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

	// Пул закрывается в Close(), поэтому дожидаемся остановки publisher'а.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		a.log.Error("Background workers did not stop in time")
	}

	a.log.Info("Server exited gracefully")
	return nil
}

// watchDatabase держит статус grpc.health в соответствии с доступностью PostgreSQL.
func (a *App) watchDatabase(ctx context.Context) {
	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()
	for {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		status := healthpb.HealthCheckResponse_SERVING
		if err := a.pool.Ping(pingCtx); err != nil {
			status = healthpb.HealthCheckResponse_NOT_SERVING
		}
		cancel()
		a.health.SetServingStatus("", status)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Close закрывает все открытые соединения и ресурсы.
func (a *App) Close() {
	if a.userConn != nil {
		_ = a.userConn.Close()
	}
	if a.producer != nil {
		_ = a.producer.Close()
	}
	if a.pool != nil {
		postgres.ClosePool(a.pool)
	}
	logger.Sync()
}
