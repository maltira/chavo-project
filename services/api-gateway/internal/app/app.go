package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	authv1 "github.com/maltira/chavo-project-backend/proto/gen/go/auth/v1"
	conversationv1 "github.com/maltira/chavo-project-backend/proto/gen/go/conversation/v1"
	userv1 "github.com/maltira/chavo-project-backend/proto/gen/go/user/v1"
	"github.com/maltira/chavo-project-backend/services/api-gateway/config"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/api"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/auth"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/hub"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/profile"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/reqctx"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/ws"
	"github.com/maltira/chavo-project-backend/services/api-gateway/pkg/logger"
	appredis "github.com/maltira/chavo-project-backend/services/api-gateway/pkg/redis"
)

type App struct {
	cfg    *config.Config
	log    *zap.Logger
	rdb    *redis.Client
	conns  []*grpc.ClientConn
	hub    *hub.Hub
	server *http.Server
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

	rdb, err := appredis.NewClient(context.Background(), cfg.RedisURL, log)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}
	a := &App{cfg: cfg, log: log, rdb: rdb}

	// grpc.NewClient не подключается сразу: сервис может стартовать позже Gateway.
	dial := func(addr string) (*grpc.ClientConn, error) {
		conn, err := grpc.NewClient(addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithUnaryInterceptor(reqctx.ClientInterceptor(cfg.GRPCTimeout)),
		)
		if err != nil {
			return nil, fmt.Errorf("grpc client %s: %w", addr, err)
		}
		a.conns = append(a.conns, conn)
		return conn, nil
	}
	authConn, err := dial(cfg.AuthAddr)
	if err != nil {
		a.Close()
		return nil, err
	}
	userConn, err := dial(cfg.UserAddr)
	if err != nil {
		a.Close()
		return nil, err
	}
	convConn, err := dial(cfg.ConversationAddr)
	if err != nil {
		a.Close()
		return nil, err
	}

	health := []api.HealthCheck{{Name: "redis", Check: func(ctx context.Context) error { return rdb.Ping(ctx).Err() }}}
	for name, conn := range map[string]*grpc.ClientConn{"auth-service": authConn, "user-service": userConn, "conversation-service": convConn} {
		client := healthpb.NewHealthClient(conn)
		health = append(health, api.HealthCheck{Name: name, Check: func(ctx context.Context) error {
			resp, err := client.Check(ctx, &healthpb.HealthCheckRequest{})
			if err != nil {
				return err
			}
			if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
				return fmt.Errorf("status %s", resp.GetStatus())
			}
			return nil
		}})
	}

	// Gateway один: записи ws:user:* от прошлого запуска принадлежат соединениям, которых уже нет.
	registry := hub.NewRedisRegistry(rdb)
	resetCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	n, err := registry.Reset(resetCtx)
	cancel()
	if err != nil {
		a.Close()
		return nil, fmt.Errorf("failed to reset ws registry: %w", err)
	}
	log.Info("Stale WS connections cleared", zap.Int("keys", n))
	a.hub = hub.New(registry, log)

	authClient := authv1.NewAuthServiceClient(authConn)
	profiles := profile.NewGate(profile.NewRedisCache(rdb), userv1.NewUserInternalServiceClient(userConn), log)
	wsHandler, err := ws.New(ws.Deps{
		Auth: authClient, Profiles: profiles, Hub: a.hub,
		Origin: cfg.FrontendOrigin, Options: ws.DefaultOptions(), Log: log,
	})
	if err != nil {
		a.Close()
		return nil, err
	}

	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router, err := api.NewRouter(api.Deps{
		Auth:           authClient,
		Users:          userv1.NewUserServiceClient(userConn),
		Conversations:  conversationv1.NewConversationServiceClient(convConn),
		Verifier:       auth.NewVerifier(cfg.JWTSecret, auth.NewRedisSessions(rdb)),
		Profiles:       profiles,
		Health:         health,
		WS:             wsHandler.Handle,
		TrustedProxies: cfg.TrustedProxies,
		Log:            log,
	})
	if err != nil {
		a.Close()
		return nil, fmt.Errorf("failed to build router: %w", err)
	}

	a.server = &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return a, nil
}

func Run() error {
	application, err := New()
	if err != nil {
		return err
	}
	defer application.Close()
	return application.Start()
}

// Start обслуживает HTTP до SIGINT/SIGTERM.
func (a *App) Start() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		a.log.Info("API Gateway starting", zap.String("port", a.cfg.Port))
		serverErr <- a.server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("http server failed: %w", err)
	case <-ctx.Done():
		a.log.Info("Shutting down server...")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		a.log.Error("Server forced to shutdown", zap.Error(err))
	}
	// Shutdown не трогает hijacked-соединения: WebSocket закрывает Hub (код 1001), снимая их с учёта в Redis.
	if err := a.hub.Shutdown(shutdownCtx); err != nil {
		a.log.Error("WS connections not closed in time", zap.Error(err))
	}

	a.log.Info("Server exited gracefully")
	return nil
}

func (a *App) Close() {
	for _, conn := range a.conns {
		_ = conn.Close()
	}
	if a.rdb != nil {
		appredis.Close(a.rdb, a.log)
	}
	logger.Sync()
}
