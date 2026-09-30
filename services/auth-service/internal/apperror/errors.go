package apperror

import (
	"errors"
	"net/http"
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
)

func HTTPCode(err error) int {
	switch {
	case errors.Is(err, ErrEmailExists):
		return http.StatusConflict
	case errors.Is(err, ErrInvalidCredentials), errors.Is(err, ErrWrongPassword):
		return http.StatusUnauthorized
	case errors.Is(err, ErrAccountNotVerified):
		return http.StatusForbidden
	case errors.Is(err, ErrInvalidOTP),
		errors.Is(err, ErrOTPAttemptsExceeded), errors.Is(err, ErrInvalidChallenge),
		errors.Is(err, ErrInvalidToken), errors.Is(err, ErrSamePassword):
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
	default:
		return "Внутренняя ошибка сервера"
	}
}
