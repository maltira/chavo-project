package apperror

import (
	"errors"
	"net/http"
)

var (
	ErrNotFound              = errors.New("not found")
	ErrProfileAlreadyExists  = errors.New("profile already exists")
	ErrUsernameExists        = errors.New("username already exists")
	ErrInvalidUUID           = errors.New("invalid UUID")
	ErrIncorrectData      = errors.New("incorrect data")
	ErrForbidden          = errors.New("forbidden")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrSelfBlock          = errors.New("cannot block yourself")
	ErrSelfUnblock        = errors.New("cannot unblock yourself")
	ErrNoColumnsToUpdate  = errors.New("no columns to update")
	ErrInvalidUsername    = errors.New("username must be 3–32 characters and contain only letters, digits, and underscores")
	ErrInvalidDisplayName = errors.New("display name must be between 1 and 100 characters")
	ErrInvalidBio         = errors.New("bio must be at most 255 characters")
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
		errors.Is(err, ErrInvalidBio):
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
	default:
		return "Внутренняя ошибка сервера"
	}
}
