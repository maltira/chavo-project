package api

import (
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/reqctx"
)

const refreshCookie = "refresh_token"

// queryInt читает целый query-параметр; некорректное значение = 0, сервис подставит значение по умолчанию.
func queryInt(c *gin.Context, name string) int32 {
	v, err := strconv.ParseInt(c.Query(name), 10, 32)
	if err != nil {
		return 0
	}
	return int32(v)
}

func optString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func userID(c *gin.Context) string {
	return reqctx.From(c.Request.Context()).UserID
}

// proxies — доверенные прокси (nginx); только от них принимаются X-Forwarded-*.
type proxies []netip.Prefix

func (p proxies) trusted(remoteAddr string) bool {
	ap, err := netip.ParseAddrPort(remoteAddr)
	if err != nil {
		return false
	}
	addr := ap.Addr().Unmap()
	for _, pr := range p {
		if pr.Contains(addr) {
			return true
		}
	}
	return false
}

// secure: cookie с флагом Secure, если клиент пришёл по HTTPS (напрямую или через доверенный прокси).
func (p proxies) secure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return r.Header.Get("X-Forwarded-Proto") == "https" && p.trusted(r.RemoteAddr)
}

func (h *Handler) setRefreshCookie(c *gin.Context, token string, expires *timestamppb.Timestamp) {
	exp := expires.AsTime()
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshCookie,
		Value:    token,
		Path:     "/",
		Expires:  exp,
		MaxAge:   max(int(time.Until(exp).Seconds()), 1),
		HttpOnly: true,
		Secure:   h.proxies.secure(c.Request),
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *Handler) clearRefreshCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.proxies.secure(c.Request),
		SameSite: http.SameSiteStrictMode,
	})
}
