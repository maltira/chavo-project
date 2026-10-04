package router

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/handler"
)

// SetupRouter: ping проверяет зависимости (БД) для GET /health.
func SetupRouter(convH *handler.ConversationHandler, groupH *handler.GroupHandler, joinH *handler.JoinHandler, msgH *handler.MessageHandler, ping func(context.Context) error) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	conversations := r.Group("/conversations")
	{
		conversations.GET("", convH.List)
		conversations.GET("/search", convH.SearchGroups)
		conversations.POST("", groupH.Create)
		conversations.GET("/:id", convH.Get)
		conversations.PATCH("/:id", groupH.Update)
		conversations.DELETE("/:id", groupH.Delete)
		conversations.POST("/:id/join", groupH.Join)

		conversations.POST("/:id/invite-link/regenerate", joinH.RegenerateInvite)
		conversations.POST("/:id/join-requests", joinH.RequestJoin)
		conversations.GET("/:id/join-requests", joinH.ListRequests)
		conversations.POST("/:id/join-requests/:request_id/approve", joinH.Approve)
		conversations.POST("/:id/join-requests/:request_id/reject", joinH.Reject)

		conversations.GET("/:id/members", groupH.ListMembers)
		conversations.POST("/:id/members", groupH.AddMembers)
		conversations.POST("/:id/members/:user_id", groupH.SetRole)
		conversations.DELETE("/:id/members/me", groupH.Leave)
		conversations.DELETE("/:id/members/:user_id", groupH.RemoveMember)

		conversations.GET("/:id/bans", groupH.ListBans)
		conversations.DELETE("/:id/bans/:user_id", groupH.Unban)

		conversations.POST("/:id/read", msgH.MarkRead)
		conversations.GET("/:id/messages/:message_id/readers", msgH.Readers)
	}

	r.GET("/invites/:token", joinH.InviteInfo)

	messages := r.Group("/messages")
	{
		messages.GET("", msgH.List)
		messages.POST("", msgH.Send)
		messages.GET("/:message_id", msgH.Get)
		messages.PATCH("/:message_id", msgH.Edit)
		messages.DELETE("/:message_id", msgH.Delete)
	}

	return r
}
