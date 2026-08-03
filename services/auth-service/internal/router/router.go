package router

import (
	"github.com/gin-gonic/gin"

	handler "github.com/maltira/chavo-project-backend/services/auth-service/internal/handler/http"
)

func SetupRouter(
	authH *handler.AuthHandler,
	otpH *handler.OtpHandler,
	refreshH *handler.RefreshHandler,
) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.ForwardedByClientIP = true

	auth := r.Group("/api/auth")
	{
		// Registration & login
		auth.POST("/register", authH.Register)
		auth.PUT("/register/verify", authH.VerifyEmail)
		auth.POST("/login", authH.Login)
		auth.PUT("/login/verify", otpH.VerifyLoginOTP)

		// Token refresh
		auth.POST("/refresh", refreshH.Refresh)

		// Logout
		auth.POST("/logout", authH.LogoutCurrent)
		auth.POST("/logout/all", authH.LogoutAll)
		auth.POST("/logout/:token_id", refreshH.TerminateSession)

		// Password reset (unauthenticated)
		auth.POST("/forgot-password", authH.ForgotPassword)
		auth.POST("/reset-password", authH.ResetPassword)

		// Profile
		auth.GET("/me", authH.Me)

		// Change password/email (authenticated)
		auth.POST("/change/pass", authH.ChangePass)
		auth.PUT("/change/pass/verify", otpH.VerifyChangePasswordOTP)
		auth.POST("/change/email", authH.ChangeEmail)
		auth.PUT("/change/email/verify", otpH.VerifyChangeMailOTP)

		// Account deletion
		auth.POST("/account/delete", authH.DeleteAccount)
		auth.POST("/account/delete/verify", otpH.VerifyDeleteAccountOTP)

		// Sessions
		auth.GET("/sessions", refreshH.ListSessions)
	}

	return r
}
