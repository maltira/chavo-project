package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	authv1 "github.com/maltira/chavo-project-backend/proto/gen/go/auth/v1"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/httpx"
)

const msgPasswordChanged = "Пароль успешно изменён. Выполните вход с новым паролем"

func (h *Handler) client(c *gin.Context) *authv1.ClientInfo {
	return &authv1.ClientInfo{Ip: c.ClientIP(), UserAgent: c.Request.UserAgent()}
}

func (h *Handler) tokens(c *gin.Context, t *authv1.TokenPair) {
	h.setRefreshCookie(c, t.GetRefreshToken(), t.GetRefreshExpiresAt())
	c.JSON(http.StatusOK, tokenJSON{AccessToken: t.GetAccessToken(), TokenType: "Bearer"})
}

func (h *Handler) register(c *gin.Context) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	if _, err := h.auth.Register(c.Request.Context(), &authv1.RegisterRequest{Email: body.Email, Password: body.Password}); err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, httpx.MessageBody{Message: "Ссылка для подтверждения отправлена на указанную почту"})
}

func (h *Handler) verifyRegister(c *gin.Context) {
	if _, err := h.auth.VerifyRegister(c.Request.Context(), &authv1.VerifyRegisterRequest{Token: c.Query("token")}); err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, httpx.MessageBody{Message: "Аккаунт успешно подтверждён"})
}

func (h *Handler) login(c *gin.Context) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	resp, err := h.auth.Login(c.Request.Context(), &authv1.LoginRequest{Email: body.Email, Password: body.Password})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"challenge_id": resp.GetChallengeId(), "message": "OTP-код отправлен на вашу почту"})
}

func (h *Handler) verifyOTP(c *gin.Context) {
	var body struct {
		ChallengeID string `json:"challenge_id"`
		Code        string `json:"code"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	resp, err := h.auth.VerifyOTP(c.Request.Context(), &authv1.VerifyOTPRequest{
		ChallengeId: body.ChallengeID, Code: body.Code, Client: h.client(c),
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	h.tokens(c, resp.GetTokens())
}

func (h *Handler) resendOTP(c *gin.Context) {
	var body struct {
		ChallengeID string `json:"challenge_id"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	if _, err := h.auth.ResendOTP(c.Request.Context(), &authv1.ResendOTPRequest{ChallengeId: body.ChallengeID}); err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, httpx.MessageBody{Message: "Новый OTP-код отправлен на вашу почту"})
}

func (h *Handler) refresh(c *gin.Context) {
	token, err := c.Cookie(refreshCookie)
	if err != nil || token == "" {
		httpx.Unauthorized(c)
		return
	}
	resp, err := h.auth.Refresh(c.Request.Context(), &authv1.RefreshRequest{RefreshToken: token, Client: h.client(c)})
	if err != nil {
		if httpx.IsClientError(err) {
			h.clearRefreshCookie(c)
		}
		h.fail(c, err)
		return
	}
	h.tokens(c, resp.GetTokens())
}

// logout публичный: отзывает сессию по refresh-cookie, даже если access token уже истёк.
func (h *Handler) logout(c *gin.Context) {
	if token, err := c.Cookie(refreshCookie); err == nil && token != "" {
		if _, err := h.auth.Logout(c.Request.Context(), &authv1.LogoutRequest{RefreshToken: token}); err != nil && !httpx.IsClientError(err) {
			h.fail(c, err)
			return
		}
	}
	h.clearRefreshCookie(c)
	c.JSON(http.StatusOK, httpx.MessageBody{Message: "Вы вышли из аккаунта"})
}

func (h *Handler) requestReset(c *gin.Context) {
	var body struct {
		Email string `json:"email"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	if _, err := h.auth.RequestPasswordReset(c.Request.Context(), &authv1.RequestPasswordResetRequest{Email: body.Email}); err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, httpx.MessageBody{Message: "Если аккаунт существует, ссылка для сброса пароля отправлена на почту"})
}

func (h *Handler) confirmReset(c *gin.Context) {
	var body struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	if _, err := h.auth.ConfirmPasswordReset(c.Request.Context(), &authv1.ConfirmPasswordResetRequest{
		Token: body.Token, Password: body.Password,
	}); err != nil {
		h.fail(c, err)
		return
	}
	h.clearRefreshCookie(c)
	c.JSON(http.StatusOK, httpx.MessageBody{Message: msgPasswordChanged})
}

func (h *Handler) changePassword(c *gin.Context) {
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	if _, err := h.auth.ChangePassword(c.Request.Context(), &authv1.ChangePasswordRequest{
		CurrentPassword: body.CurrentPassword, NewPassword: body.NewPassword,
	}); err != nil {
		h.fail(c, err)
		return
	}
	h.clearRefreshCookie(c)
	c.JSON(http.StatusOK, httpx.MessageBody{Message: msgPasswordChanged})
}

func (h *Handler) listSessions(c *gin.Context) {
	resp, err := h.auth.ListSessions(c.Request.Context(), &authv1.ListSessionsRequest{})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, mapSlice(resp.GetSessions(), toSession))
}

func (h *Handler) terminateSession(c *gin.Context) {
	if _, err := h.auth.TerminateSession(c.Request.Context(), &authv1.TerminateSessionRequest{
		SessionId: c.Param("session_id"),
	}); err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, httpx.MessageBody{Message: "Сессия завершена"})
}
