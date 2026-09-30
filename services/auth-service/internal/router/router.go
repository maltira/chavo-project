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

	auth := r.Group("/auth")
	{
		// Registration
		auth.POST("/register", authH.Register)
		auth.GET("/register/verify", authH.VerifyRegister)

		// Login & OTP
		auth.POST("/login", authH.Login)
		auth.POST("/otp/verify", otpH.VerifyOTP)
		auth.POST("/otp/resend", otpH.ResendOTP)

		// Token refresh & Logout
		auth.POST("/refresh", refreshH.Refresh)
		auth.POST("/logout", authH.Logout)

		// Password reset & change
		auth.POST("/reset/request", authH.RequestPasswordReset)
		auth.POST("/reset/confirm", authH.ConfirmPasswordReset)
		auth.POST("/change-password", authH.ChangePassword)

		// Sessions
		auth.GET("/sessions", refreshH.ListSessions)
		auth.DELETE("/sessions/:session_id", refreshH.TerminateSession)
	}

	return r
}
