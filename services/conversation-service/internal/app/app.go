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
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/client/userclient"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/handler"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/outbox"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/router"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
	"github.com/maltira/chavo-project-backend/services/conversation-service/pkg/crypto"
	pkgkafka "github.com/maltira/chavo-project-backend/services/conversation-service/pkg/kafka"
	"github.com/maltira/chavo-project-backend/services/conversation-service/pkg/logger"
	"github.com/maltira/chavo-project-backend/services/conversation-service/pkg/postgres"
	"go.uber.org/zap"
)

type App struct {
	cfg       *config.Config
	log       *zap.Logger
	pool      *pgxpool.Pool
	producer  *pkgkafka.Producer
	publisher *outbox.Publisher
	server    *http.Server
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

	cipher, err := crypto.NewCipher(cfg.EncryptionKey)
	if err != nil {
		postgres.ClosePool(pool)
		return nil, fmt.Errorf("failed to init cipher: %w", err)
	}

	db := repository.NewDB(pool)
	convRepo := repository.NewConversationRepository()
	msgRepo := repository.NewMessageRepository()
	outboxRepo := repository.NewOutboxRepository()
	joinRepo := repository.NewJoinRequestRepository()
	banRepo := repository.NewBanRepository()
	users := userclient.New(cfg.UserServiceURL)
	producer := pkgkafka.NewProducer(cfg.KafkaBrokers, log)
	publisher := outbox.NewPublisher(db, outboxRepo, producer, cipher, log, outbox.DefaultOptions())

	convSvc := service.NewConversationService(db, convRepo, cipher)
	msgSvc := service.NewMessageService(db, convRepo, msgRepo, outboxRepo, users, cipher, log)

	groupSvc := service.NewGroupService(db, convRepo, outboxRepo, users, banRepo)
	joinSvc := service.NewJoinService(db, convRepo, joinRepo, outboxRepo, banRepo)

	convH := handler.NewConversationHandler(convSvc, log)
	groupH := handler.NewGroupHandler(groupSvc, log)
	joinH := handler.NewJoinHandler(joinSvc, log)
	msgH := handler.NewMessageHandler(msgSvc, log)

	r := router.SetupRouter(convH, groupH, joinH, msgH)
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	return &App{
		cfg:       cfg,
		log:       log,
		pool:      pool,
		producer:  producer,
		publisher: publisher,
		server:    server,
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

	publisherDone := make(chan struct{})
	go func() {
		defer close(publisherDone)
		a.publisher.Run(srvCtx)
	}()
	// Пул закрывается в Close() после Start(), поэтому дожидаемся остановки publisher'а.
	defer func() {
		stop()
		select {
		case <-publisherDone:
		case <-time.After(15 * time.Second):
			a.log.Error("Outbox publisher did not stop in time")
		}
	}()

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
	if a.producer != nil {
		_ = a.producer.Close()
	}
	if a.pool != nil {
		postgres.ClosePool(a.pool)
	}
	logger.Sync()
}
