package apperror

import (
	"errors"
	"net/http"

	"github.com/maltira/chavo-project-backend/proto/grpcx"
	"github.com/maltira/chavo-project-backend/proto/status"
)

var (
	ErrNotFound             = errors.New("not found")
	ErrProfileAlreadyExists = errors.New("profile already exists")
	ErrUsernameExists       = errors.New("username already exists")
	ErrInvalidUUID          = errors.New("invalid UUID")
	ErrIncorrectData        = errors.New("incorrect data")
	ErrForbidden            = errors.New("forbidden")
	ErrUnauthorized         = errors.New("unauthorized")
	ErrSelfBlock            = errors.New("cannot block yourself")
	ErrSelfUnblock          = errors.New("cannot unblock yourself")
	ErrNoColumnsToUpdate    = errors.New("no columns to update")
	ErrInvalidUsername      = errors.New("username must be 3–32 characters and contain only letters, digits, and underscores")
	ErrInvalidDisplayName   = errors.New("display name must be between 1 and 100 characters")
	ErrInvalidBio           = errors.New("bio must be at most 255 characters")
)

func HTTPCode(err error) int {
	switch {
	case errors.Is(err, ErrProfileAlreadyExists),
		errors.Is(err, ErrUsernameExists):
		return http.StatusConflict
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrInvalidUUID),
		errors.Is(err, ErrIncorrectData),
		errors.Is(err, ErrSelfBlock),
		errors.Is(err, ErrSelfUnblock),
		errors.Is(err, ErrNoColumnsToUpdate),
		errors.Is(err, ErrInvalidUsername),
		errors.Is(err, ErrInvalidDisplayName),
		errors.Is(err, ErrInvalidBio),
		errors.Is(err, status.ErrUnknownIcon),
		errors.Is(err, status.ErrUnknownDuration),
		errors.Is(err, status.ErrInvalidText),
		errors.Is(err, status.ErrInvalidColor):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func UserMessage(err error) string {
	switch {
	case errors.Is(err, ErrProfileAlreadyExists):
		return "Профиль уже создан"
	case errors.Is(err, ErrUsernameExists):
		return "Имя пользователя уже занято"
	case errors.Is(err, ErrNotFound):
		return "Запись не найдена"
	case errors.Is(err, ErrForbidden):
		return "Недостаточно прав"
	case errors.Is(err, ErrUnauthorized):
		return "Необходима авторизация"
	case errors.Is(err, ErrInvalidUUID):
		return "Некорректный формат UUID"
	case errors.Is(err, ErrIncorrectData):
		return "Некорректные входные данные"
	case errors.Is(err, ErrSelfBlock):
		return "Нельзя заблокировать свой профиль"
	case errors.Is(err, ErrSelfUnblock):
		return "Нельзя разблокировать свой профиль"
	case errors.Is(err, ErrNoColumnsToUpdate):
		return "Не передано ни одного поля для обновления"
	case errors.Is(err, ErrInvalidUsername):
		return "Имя пользователя должно содержать от 3 до 32 символов (буквы, цифры, _)"
	case errors.Is(err, ErrInvalidDisplayName):
		return "Отображаемое имя должно содержать от 1 до 100 символов"
	case errors.Is(err, ErrInvalidBio):
		return "Описание должно содержать не более 255 символов"
	case errors.Is(err, status.ErrUnknownIcon):
		return "Неизвестная иконка статуса"
	case errors.Is(err, status.ErrInvalidColor):
		return "Цвета статуса задаются в формате #RRGGBB"
	case errors.Is(err, status.ErrUnknownDuration):
		return "Неизвестный срок статуса"
	case errors.Is(err, status.ErrInvalidText):
		return "Текст статуса — от 1 до 70 символов в одну строку"
	default:
		return "Внутренняя ошибка сервера"
	}
}

// Reason — машинный код ошибки для клиента ("" — без кода).
func Reason(err error) string {
	switch {
	case errors.Is(err, ErrProfileAlreadyExists):
		return "PROFILE_ALREADY_EXISTS"
	case errors.Is(err, ErrUsernameExists):
		return "USERNAME_TAKEN"
	case errors.Is(err, ErrNotFound):
		return "NOT_FOUND"
	case errors.Is(err, status.ErrUnknownIcon):
		return "STATUS_ICON_INVALID"
	case errors.Is(err, status.ErrInvalidColor):
		return "STATUS_COLOR_INVALID"
	case errors.Is(err, status.ErrUnknownDuration):
		return "STATUS_DURATION_INVALID"
	case errors.Is(err, status.ErrInvalidText):
		return "STATUS_TEXT_INVALID"
	default:
		return ""
	}
}

// GRPCStatus переводит ошибку в gRPC status по той же таблице, что и HTTP-коды; текст внутренних ошибок не раскрывается.
func GRPCStatus(err error) error {
	return grpcx.Error(grpcx.CodeFromHTTP(HTTPCode(err)), Reason(err), UserMessage(err))
}
