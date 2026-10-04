package router

import (
	"github.com/gin-gonic/gin"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/handler"
)

func SetupRouter(convH *handler.ConversationHandler, groupH *handler.GroupHandler, joinH *handler.JoinHandler, msgH *handler.MessageHandler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

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
