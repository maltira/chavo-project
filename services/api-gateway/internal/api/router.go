// Package api — внешний HTTP API: разбор запроса → gRPC-вызов сервиса → JSON прежнего формата.
package api

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	authv1 "github.com/maltira/chavo-project-backend/proto/gen/go/auth/v1"
	conversationv1 "github.com/maltira/chavo-project-backend/proto/gen/go/conversation/v1"
	userv1 "github.com/maltira/chavo-project-backend/proto/gen/go/user/v1"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/auth"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/httpx"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/middleware"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/profile"
)

const maxBodyBytes = 1 << 20

// HealthCheck — проверка зависимости для GET /health.
type HealthCheck struct {
	Name  string
	Check func(ctx context.Context) error
}

type Deps struct {
	Auth          authv1.AuthServiceClient
	Users         userv1.UserServiceClient
	Conversations conversationv1.ConversationServiceClient
	Verifier      *auth.Verifier
	Profiles      *profile.Gate
	Health        []HealthCheck
	// TrustedProxies — IP или CIDR; пусто = не доверять никому.
	TrustedProxies []string
	Log            *zap.Logger
}

type Handler struct {
	auth          authv1.AuthServiceClient
	users         userv1.UserServiceClient
	conversations conversationv1.ConversationServiceClient
	profiles      *profile.Gate
	health        []HealthCheck
	proxies       proxies
	log           *zap.Logger
}

func (h *Handler) fail(c *gin.Context, err error) {
	httpx.GRPCError(c, h.log, err)
}

func parseProxies(list []string) (proxies, error) {
	res := make(proxies, 0, len(list))
	for _, s := range list {
		if p, err := netip.ParsePrefix(s); err == nil {
			res = append(res, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(s)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy %q", s)
		}
		a = a.Unmap()
		res = append(res, netip.PrefixFrom(a, a.BitLen()))
	}
	return res, nil
}

func NewRouter(d Deps) (*gin.Engine, error) {
	px, err := parseProxies(d.TrustedProxies)
	if err != nil {
		return nil, err
	}
	h := &Handler{
		auth: d.Auth, users: d.Users, conversations: d.Conversations,
		profiles: d.Profiles, health: d.Health, proxies: px, log: d.Log,
	}

	r := gin.New()
	// Без доверенных прокси ClientIP = адрес соединения; X-Forwarded-For принимается только от nginx.
	if err := r.SetTrustedProxies(d.TrustedProxies); err != nil {
		return nil, err
	}
	r.RemoteIPHeaders = []string{"X-Forwarded-For"}
	r.HandleMethodNotAllowed = true
	r.NoRoute(func(c *gin.Context) { httpx.Abort(c, http.StatusNotFound, "Не найдено", httpx.ReasonNotFound) })
	r.NoMethod(func(c *gin.Context) { httpx.Abort(c, http.StatusMethodNotAllowed, "Метод не поддерживается", "") })
	r.Use(middleware.Recovery(d.Log), middleware.Trace(), middleware.Logger(d.Log), middleware.BodyLimit(maxBodyBytes))

	r.GET("/health", h.healthCheck)

	api := r.Group("/api")

	// Публичные маршруты.
	pub := api.Group("/auth")
	pub.POST("/register", h.register)
	pub.GET("/register/verify", h.verifyRegister)
	pub.POST("/login", h.login)
	pub.POST("/otp/verify", h.verifyOTP)
	pub.POST("/otp/resend", h.resendOTP)
	pub.POST("/refresh", h.refresh)
	pub.POST("/logout", h.logout)
	pub.POST("/reset/request", h.requestReset)
	pub.POST("/reset/confirm", h.confirmReset)

	// Авторизованные, профиль не нужен: управление аккаунтом и создание профиля.
	authed := api.Group("", middleware.Authenticate(d.Verifier, d.Log))
	authed.POST("/auth/change-password", h.changePassword)
	authed.GET("/auth/sessions", h.listSessions)
	authed.DELETE("/auth/sessions/:session_id", h.terminateSession)
	authed.POST("/users", h.createProfile)
	authed.GET("/users/me", h.getMe)

	// Всё остальное — только с профилем.
	p := authed.Group("", middleware.RequireProfile(d.Profiles, d.Log))

	p.GET("/users", h.searchProfiles)
	p.PATCH("/users/me", h.updateMe)
	p.GET("/users/me/blocked", h.listBlocked)
	p.GET("/users/me/settings", h.getSettings)
	p.PATCH("/users/me/settings", h.updateSettings)
	p.GET("/users/:user_id", h.getProfile)
	p.GET("/users/:user_id/block-status", h.blockStatus)
	p.POST("/users/:user_id/block", h.block)
	p.DELETE("/users/:user_id/block", h.unblock)

	conv := p.Group("/conversations")
	conv.GET("", h.listConversations)
	conv.GET("/search", h.searchGroups)
	conv.POST("", h.createGroup)
	conv.GET("/:id", h.getConversation)
	conv.PATCH("/:id", h.updateGroup)
	conv.DELETE("/:id", h.deleteConversation)
	conv.POST("/:id/join", h.joinGroup)
	conv.POST("/:id/invite-link/regenerate", h.regenerateInvite)
	conv.POST("/:id/join-requests", h.requestJoin)
	conv.GET("/:id/join-requests", h.listJoinRequests)
	conv.POST("/:id/join-requests/:request_id/approve", h.approveJoinRequest)
	conv.POST("/:id/join-requests/:request_id/reject", h.rejectJoinRequest)
	conv.GET("/:id/members", h.listMembers)
	conv.POST("/:id/members", h.addMembers)
	conv.POST("/:id/members/:user_id", h.setMemberRole)
	conv.DELETE("/:id/members/:user_id", h.removeMember)
	conv.GET("/:id/bans", h.listBans)
	conv.DELETE("/:id/bans/:user_id", h.unban)
	conv.POST("/:id/read", h.markRead)
	conv.GET("/:id/messages/:message_id/readers", h.listReaders)

	p.GET("/invites/:token", h.inviteInfo)

	msg := p.Group("/messages")
	msg.GET("", h.listMessages)
	msg.POST("", h.sendMessage)
	msg.GET("/:message_id", h.getMessage)
	msg.PATCH("/:message_id", h.editMessage)
	msg.DELETE("/:message_id", h.deleteMessage)

	return r, nil
}

func (h *Handler) healthCheck(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	for _, hc := range h.health {
		if err := hc.Check(ctx); err != nil {
			h.log.Warn("health check failed", zap.String("dependency", hc.Name), zap.Error(err))
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
