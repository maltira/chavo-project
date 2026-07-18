package apperror

import (
	"errors"
	"net/http"
)

// Sentinel errors used across the application.
var (
	ErrEmailExists        = errors.New("email already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountNotVerified = errors.New("account not verified")
	ErrInvalidOTP         = errors.New("invalid or expired OTP")
	ErrOTPTypeMismatch    = errors.New("OTP code type mismatch")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrNotFound           = errors.New("not found")
	ErrForbidden          = errors.New("forbidden")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrSameEmail          = errors.New("new email must differ from current")
	ErrWrongPassword      = errors.New("wrong password")
)

// HTTPCode maps a sentinel error to the appropriate HTTP status code.
func HTTPCode(err error) int {
	switch {
	case errors.Is(err, ErrEmailExists), errors.Is(err, ErrSameEmail):
		return http.StatusConflict
	case errors.Is(err, ErrInvalidCredentials), errors.Is(err, ErrWrongPassword):
		return http.StatusUnauthorized
	case errors.Is(err, ErrAccountNotVerified):
		return http.StatusForbidden
	case errors.Is(err, ErrInvalidOTP), errors.Is(err, ErrInvalidToken), errors.Is(err, ErrOTPTypeMismatch):
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

// UserMessage returns a user-facing localized message for the given error.
func UserMessage(err error) string {
	switch {
	case errors.Is(err, ErrEmailExists):
		return "Пользователь с такой почтой уже существует"
	case errors.Is(err, ErrInvalidCredentials):
		return "Неверный email или пароль"
	case errors.Is(err, ErrAccountNotVerified):
		return "Аккаунт не подтверждён"
	case errors.Is(err, ErrInvalidOTP), errors.Is(err, ErrOTPTypeMismatch):
		return "Неверный или истекший OTP-код"
	case errors.Is(err, ErrInvalidToken):
		return "Недействительная или истекшая ссылка"
	case errors.Is(err, ErrNotFound):
		return "Запись не найдена"
	case errors.Is(err, ErrForbidden):
		return "Недостаточно прав"
	case errors.Is(err, ErrUnauthorized):
		return "Необходима авторизация"
	case errors.Is(err, ErrSameEmail):
		return "Новый email не должен совпадать с текущим"
	case errors.Is(err, ErrWrongPassword):
		return "Указан неверный пароль"
	default:
		return "Внутренняя ошибка сервера"
	}
}
