package apperror

import (
	"errors"
	"net/http"

	"github.com/maltira/chavo-project-backend/proto/grpcx"
)

var (
	ErrEmailExists         = errors.New("email already exists")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrAccountNotVerified  = errors.New("account not verified")
	ErrInvalidOTP          = errors.New("invalid OTP")
	ErrOTPAttemptsExceeded = errors.New("OTP attempts exceeded")
	ErrInvalidChallenge    = errors.New("invalid or expired challenge")
	ErrInvalidToken        = errors.New("invalid or expired token")
	ErrNotFound            = errors.New("not found")
	ErrForbidden           = errors.New("forbidden")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrWrongPassword       = errors.New("wrong password")
	ErrSamePassword        = errors.New("new password cannot be the same as old password")
	ErrIncorrectData       = errors.New("incorrect data")
	ErrTOTPNotPending      = errors.New("totp setup expired")
	ErrTOTPNotEnabled      = errors.New("totp not enabled")
	ErrTOTPAlreadyEnabled  = errors.New("totp already enabled")
	ErrResendNotAllowed    = errors.New("resend not allowed for totp")
)

func HTTPCode(err error) int {
	switch {
	case errors.Is(err, ErrIncorrectData):
		return http.StatusBadRequest
	case errors.Is(err, ErrEmailExists), errors.Is(err, ErrTOTPAlreadyEnabled):
		return http.StatusConflict
	case errors.Is(err, ErrInvalidCredentials), errors.Is(err, ErrWrongPassword):
		return http.StatusUnauthorized
	case errors.Is(err, ErrAccountNotVerified):
		return http.StatusForbidden
	case errors.Is(err, ErrInvalidOTP),
		errors.Is(err, ErrOTPAttemptsExceeded), errors.Is(err, ErrInvalidChallenge),
		errors.Is(err, ErrInvalidToken), errors.Is(err, ErrSamePassword),
		errors.Is(err, ErrTOTPNotPending), errors.Is(err, ErrTOTPNotEnabled),
		errors.Is(err, ErrResendNotAllowed):
		return http.StatusBadRequest
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

func UserMessage(err error) string {
	switch {
	case errors.Is(err, ErrIncorrectData):
		return "Некорректные входные данные"
	case errors.Is(err, ErrEmailExists):
		return "Пользователь с такой почтой уже существует"
	case errors.Is(err, ErrInvalidCredentials):
		return "Неверный email или пароль"
	case errors.Is(err, ErrAccountNotVerified):
		return "Аккаунт не подтверждён"
	case errors.Is(err, ErrInvalidOTP):
		return "Неверный OTP-код"
	case errors.Is(err, ErrOTPAttemptsExceeded):
		return "Превышено количество попыток ввода OTP"
	case errors.Is(err, ErrInvalidChallenge):
		return "Недействительная или устаревшая сессия входа"
	case errors.Is(err, ErrInvalidToken):
		return "Недействительная или истекшая ссылка"
	case errors.Is(err, ErrNotFound):
		return "Запись не найдена"
	case errors.Is(err, ErrForbidden):
		return "Недостаточно прав"
	case errors.Is(err, ErrUnauthorized):
		return "Необходима авторизация"
	case errors.Is(err, ErrWrongPassword):
		return "Указан неверный текущий пароль"
	case errors.Is(err, ErrSamePassword):
		return "Новый пароль не может совпадать с текущим"
	case errors.Is(err, ErrTOTPNotPending):
		return "Сеанс настройки аутентификатора истёк, начните заново"
	case errors.Is(err, ErrTOTPNotEnabled):
		return "Приложение-аутентификатор не подключено"
	case errors.Is(err, ErrTOTPAlreadyEnabled):
		return "Приложение-аутентификатор уже подключено"
	case errors.Is(err, ErrResendNotAllowed):
		return "Повторная отправка доступна только для кодов на почту"
	default:
		return "Внутренняя ошибка сервера"
	}
}

// Reason — машинный код ошибки для клиента ("" — без кода).
func Reason(err error) string {
	switch {
	case errors.Is(err, ErrEmailExists):
		return "EMAIL_EXISTS"
	case errors.Is(err, ErrInvalidCredentials):
		return "INVALID_CREDENTIALS"
	case errors.Is(err, ErrAccountNotVerified):
		return "ACCOUNT_NOT_VERIFIED"
	case errors.Is(err, ErrInvalidOTP):
		return "INVALID_OTP"
	case errors.Is(err, ErrOTPAttemptsExceeded):
		return "OTP_ATTEMPTS_EXCEEDED"
	case errors.Is(err, ErrInvalidChallenge):
		return "INVALID_CHALLENGE"
	case errors.Is(err, ErrInvalidToken):
		return "INVALID_TOKEN"
	case errors.Is(err, ErrWrongPassword):
		return "WRONG_PASSWORD"
	case errors.Is(err, ErrSamePassword):
		return "SAME_PASSWORD"
	case errors.Is(err, ErrTOTPNotPending):
		return "TOTP_SETUP_EXPIRED"
	case errors.Is(err, ErrTOTPNotEnabled):
		return "TOTP_NOT_ENABLED"
	case errors.Is(err, ErrTOTPAlreadyEnabled):
		return "TOTP_ALREADY_ENABLED"
	case errors.Is(err, ErrResendNotAllowed):
		return "RESEND_NOT_ALLOWED"
	default:
		return ""
	}
}

// GRPCStatus переводит ошибку в gRPC status по тем же таблицам, что и HTTP-коды; текст внутренних ошибок не раскрывается.
func GRPCStatus(err error) error {
	return grpcx.Error(grpcx.CodeFromHTTP(HTTPCode(err)), Reason(err), UserMessage(err))
}
