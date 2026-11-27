package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	authv1 "github.com/maltira/chavo-project-backend/proto/gen/go/auth/v1"
	userv1 "github.com/maltira/chavo-project-backend/proto/gen/go/user/v1"
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
	method := resp.GetMethod()
	if method == "" {
		method = "email"
	}
	msg := "OTP-код отправлен на вашу почту"
	if method == "totp" {
		msg = "Введите код из приложения-аутентификатора"
	}
	c.JSON(http.StatusOK, gin.H{"challenge_id": resp.GetChallengeId(), "method": method, "message": msg})
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

func (h *Handler) getTwoFactor(c *gin.Context) {
	resp, err := h.auth.GetTwoFactor(c.Request.Context(), &authv1.GetTwoFactorRequest{})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"method":                 resp.GetMethod(),
		"backup_codes_remaining": resp.GetBackupCodesRemaining(),
	})
}

func (h *Handler) setupTOTP(c *gin.Context) {
	var body struct {
		Password string `json:"password"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	resp, err := h.auth.SetupTOTP(c.Request.Context(), &authv1.SetupTOTPRequest{Password: body.Password})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"secret":      resp.GetSecret(),
		"otpauth_url": resp.GetOtpauthUrl(),
	})
}

func (h *Handler) confirmTOTP(c *gin.Context) {
	var body struct {
		Code string `json:"code"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	resp, err := h.auth.ConfirmTOTP(c.Request.Context(), &authv1.ConfirmTOTPRequest{Code: body.Code})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"backup_codes": nonNil(resp.GetBackupCodes())})
}

func (h *Handler) switchTwoFactorMethod(c *gin.Context) {
	var body struct {
		Method   string `json:"method"`
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	if _, err := h.auth.SwitchTwoFactorMethod(c.Request.Context(), &authv1.SwitchTwoFactorMethodRequest{
		Method: body.Method, Password: body.Password, Code: body.Code,
	}); err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, httpx.MessageBody{Message: "Способ подтверждения входа изменён"})
}

func (h *Handler) regenerateBackupCodes(c *gin.Context) {
	var body struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	resp, err := h.auth.RegenerateBackupCodes(c.Request.Context(), &authv1.RegenerateBackupCodesRequest{
		Password: body.Password, Code: body.Code,
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"backup_codes": nonNil(resp.GetBackupCodes())})
}

func (h *Handler) deleteAccount(c *gin.Context) {
	var body struct {
		Password string  `json:"password"`
		Code     *string `json:"code"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	uid := userID(c)
	// собираем адреса аватарок заранее, чтобы удалить файлы из MinIO после подтверждения удаления аккаунта
	var avatarURLs []string
	if h.media != nil && h.users != nil && uid != "" {
		if list, err := h.users.ListAvatars(c.Request.Context(), &userv1.ListAvatarsRequest{UserId: uid}); err == nil {
			for _, item := range list.GetItems() {
				avatarURLs = append(avatarURLs, item.GetAvatarUrl())
			}
		}
	}
	if _, err := h.auth.DeleteAccount(c.Request.Context(), &authv1.DeleteAccountRequest{
		Password: body.Password, Code: body.Code,
	}); err != nil {
		h.fail(c, err)
		return
	}
	h.clearRefreshCookie(c)
	if h.profiles != nil && uid != "" {
		_ = h.profiles.Deleted(c.Request.Context(), uid)
	}
	for _, u := range avatarURLs {
		h.dropAvatar(u)
	}
	c.JSON(http.StatusOK, httpx.MessageBody{Message: "Аккаунт удалён"})
}
