package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/auth"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/httpx"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/profile"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/reqctx"
)

const (
	HeaderRequestID   = "X-Request-ID"
	HeaderTraceParent = "traceparent"
)

var (
	requestIDPattern   = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)
	traceParentPattern = regexp.MustCompile(`^00-([0-9a-f]{32})-[0-9a-f]{16}-([0-9a-f]{2})$`)
)

const zeroTraceID = "00000000000000000000000000000000"

// Trace принимает X-Request-ID и W3C traceparent клиента (или создаёт их), открывает span Gateway
// и возвращает оба заголовка в ответе.
func Trace() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(HeaderRequestID)
		if !requestIDPattern.MatchString(requestID) {
			requestID = uuid.NewString()
		}

		traceID, flags := "", "01"
		if m := traceParentPattern.FindStringSubmatch(c.GetHeader(HeaderTraceParent)); m != nil && m[1] != zeroTraceID {
			traceID, flags = m[1], m[2]
		} else {
			traceID = randomHex(16)
		}
		traceParent := "00-" + traceID + "-" + randomHex(8) + "-" + flags

		info := &reqctx.Info{RequestID: requestID, TraceID: traceID, TraceParent: traceParent}
		c.Request = c.Request.WithContext(reqctx.With(c.Request.Context(), info))
		c.Header(HeaderRequestID, requestID)
		c.Header(HeaderTraceParent, traceParent)
		c.Next()
	}
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Logger пишет одну строку на запрос. Путь — шаблон маршрута: query (токены подтверждения) и тело не логируются.
func Logger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		info := reqctx.From(c.Request.Context())
		endpoint := c.FullPath()
		if endpoint == "" {
			endpoint = "unmatched"
		}
		status := c.Writer.Status()
		fields := []zap.Field{
			zap.String("request_id", info.RequestID),
			zap.String("trace_id", info.TraceID),
			zap.String("user_id", info.UserID),
			zap.String("session_id", info.SessionID),
			zap.String("endpoint", c.Request.Method+" "+endpoint),
			zap.Int("status", status),
			zap.Duration("latency", time.Since(start)),
		}
		if code := c.GetString(httpx.KeyErrorCode); code != "" {
			fields = append(fields, zap.String("error_code", code))
		}
		if status >= http.StatusInternalServerError {
			log.Error("request", fields...)
		} else {
			log.Info("request", fields...)
		}
	}
}

func Recovery(log *zap.Logger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, recovered any) {
		log.Error("panic", zap.Any("panic", recovered), zap.String("endpoint", c.FullPath()),
			zap.String("request_id", reqctx.From(c.Request.Context()).RequestID))
		httpx.Internal(c)
	})
}

func BodyLimit(n int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, n)
		c.Next()
	}
}

// Authenticate проверяет access token; X-User-ID клиента нигде не читается — личность берётся только из токена.
func Authenticate(v *auth.Verifier, log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, sid, err := v.Verify(c.Request.Context(), c.GetHeader("Authorization"))
		if errors.Is(err, auth.ErrUnauthorized) {
			httpx.Unauthorized(c)
			return
		}
		if err != nil {
			log.Error("session check failed", zap.Error(err))
			httpx.Unavailable(c)
			return
		}
		info := reqctx.From(c.Request.Context())
		info.UserID, info.SessionID = userID, sid
		c.Next()
	}
}

// RequireProfile пускает дальше только пользователей с созданным профилем.
func RequireProfile(g *profile.Gate, log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ok, err := g.Exists(c.Request.Context(), reqctx.From(c.Request.Context()).UserID)
		if err != nil {
			httpx.GRPCError(c, log, err)
			return
		}
		if !ok {
			httpx.Abort(c, http.StatusForbidden, "Необходимо заполнить профиль", httpx.ReasonProfileRequired)
			return
		}
		c.Next()
	}
}
