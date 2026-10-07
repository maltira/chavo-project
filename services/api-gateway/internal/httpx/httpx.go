// Package httpx — формат ответов и ошибок внешнего API.
package httpx

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"

	"github.com/maltira/chavo-project-backend/proto/grpcx"
)

// KeyErrorCode — ключ gin-контекста с кодом ошибки для лога запроса.
const KeyErrorCode = "error_code"

const (
	ReasonUnauthorized    = "UNAUTHORIZED"
	ReasonProfileRequired = "PROFILE_REQUIRED"
	ReasonInvalidRequest  = "INVALID_REQUEST"
	ReasonTooLarge        = "PAYLOAD_TOO_LARGE"
	ReasonUnavailable     = "SERVICE_UNAVAILABLE"
	ReasonTimeout         = "TIMEOUT"
	ReasonNotFound        = "NOT_FOUND"
	ReasonRateLimited     = "RATE_LIMITED"
)

const msgInternal = "Внутренняя ошибка сервера"

type ErrorBody struct {
	Error  string `json:"error"`
	Reason string `json:"reason,omitempty"`
}

func Abort(c *gin.Context, status int, message, reason string) {
	code := reason
	if code == "" {
		code = http.StatusText(status)
	}
	c.Set(KeyErrorCode, code)
	c.AbortWithStatusJSON(status, ErrorBody{Error: message, Reason: reason})
}

func Unauthorized(c *gin.Context) {
	Abort(c, http.StatusUnauthorized, "Необходима авторизация", ReasonUnauthorized)
}

func Unavailable(c *gin.Context) {
	Abort(c, http.StatusServiceUnavailable, "Сервис временно недоступен", ReasonUnavailable)
}

func Internal(c *gin.Context) {
	Abort(c, http.StatusInternalServerError, msgInternal, "")
}

// fallbackMessages — тексты для ошибок сервиса без пользовательского сообщения.
var fallbackMessages = map[int]string{
	http.StatusBadRequest:      "Некорректные данные запроса",
	http.StatusUnauthorized:    "Необходима авторизация",
	http.StatusForbidden:       "Доступ запрещён",
	http.StatusNotFound:        "Не найдено",
	http.StatusConflict:        "Конфликт данных",
	http.StatusTooManyRequests: "Слишком много запросов",
}

// GRPCError переводит ошибку вызова сервиса в {error, reason}; внутренние детали наружу не уходят.
func GRPCError(c *gin.Context, log *zap.Logger, err error) {
	code, reason, message := grpcx.Details(err)
	status := grpcx.HTTPFromCode(code)

	switch {
	case code == codes.Canceled:
		// Клиент ушёл, отвечать некому.
		c.Set(KeyErrorCode, code.String())
		c.AbortWithStatus(499)
	case code == codes.Unavailable && message == "":
		log.Warn("service unavailable", zap.String("endpoint", c.FullPath()), zap.Error(err))
		Unavailable(c)
	case code == codes.DeadlineExceeded:
		log.Warn("service timeout", zap.String("endpoint", c.FullPath()), zap.Error(err))
		Abort(c, http.StatusGatewayTimeout, "Сервис не ответил вовремя", ReasonTimeout)
	case status >= http.StatusInternalServerError:
		log.Error("service error", zap.String("endpoint", c.FullPath()), zap.String("grpc_code", code.String()), zap.Error(err))
		Internal(c)
	default:
		if message == "" {
			message = fallbackMessages[status]
		}
		if reason == "" {
			c.Set(KeyErrorCode, code.String())
			c.AbortWithStatusJSON(status, ErrorBody{Error: message})
			return
		}
		Abort(c, status, message, reason)
	}
}

// IsClientError — сервис отклонил запрос (4xx), а не был недоступен или сломан.
func IsClientError(err error) bool {
	code, _, _ := grpcx.Details(err)
	status := grpcx.HTTPFromCode(code)
	return status >= 400 && status < 500 && code != codes.Canceled
}

// BindJSON читает тело запроса; при ошибке отвечает 400 или 413 и возвращает false.
func BindJSON(c *gin.Context, dst any) bool {
	if err := json.NewDecoder(c.Request.Body).Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			Abort(c, http.StatusRequestEntityTooLarge, "Слишком большой запрос", ReasonTooLarge)
		} else {
			Abort(c, http.StatusBadRequest, "Некорректные данные запроса", ReasonInvalidRequest)
		}
		return false
	}
	return true
}

type MessageBody struct {
	Message string `json:"message"`
}

// SuccessBody — прежний формат ответов user- и conversation-сервисов.
type SuccessBody struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func Success(c *gin.Context, status int, message string) {
	c.JSON(status, SuccessBody{Success: true, Message: message})
}
