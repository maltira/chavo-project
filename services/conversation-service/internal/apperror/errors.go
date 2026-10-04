package apperror

import (
	"errors"
	"net/http"
)

var (
	ErrNotFound              = errors.New("not found")
	ErrForbidden             = errors.New("forbidden")
	ErrUnauthorized          = errors.New("unauthorized")
	ErrInvalidUUID           = errors.New("invalid UUID")
	ErrIncorrectData         = errors.New("incorrect data")
	ErrNotMember             = errors.New("not a conversation member")
	ErrBlockedByMe           = errors.New("recipient is blocked by you")
	ErrBlockedByThem         = errors.New("you are blocked by recipient")
	ErrUserNotFound          = errors.New("user not found")
	ErrInvalidReply          = errors.New("reply message belongs to another conversation")
	ErrMessageDeleted        = errors.New("message is deleted")
	ErrAlreadyMember         = errors.New("already a member")
	ErrUserServiceError      = errors.New("user service unavailable")
	ErrLastAdmin             = errors.New("cannot leave group without admin")
	ErrInviteNotAllowed      = errors.New("user does not allow group invites")
	ErrJoinRequestExists     = errors.New("join request already pending")
	ErrJoinRequestNotPending = errors.New("join request is not pending")
	ErrBanned                = errors.New("user is banned in this group")
	ErrGroupFull             = errors.New("group member limit reached")
	ErrJoinRequestsLimit     = errors.New("too many pending join requests")
)

func HTTPCode(err error) int {
	switch {
	case errors.Is(err, ErrNotFound),
		errors.Is(err, ErrUserNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrForbidden),
		errors.Is(err, ErrNotMember),
		errors.Is(err, ErrBlockedByMe),
		errors.Is(err, ErrBlockedByThem),
		errors.Is(err, ErrInviteNotAllowed),
		errors.Is(err, ErrBanned):
		return http.StatusForbidden
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrAlreadyMember),
		errors.Is(err, ErrLastAdmin),
		errors.Is(err, ErrJoinRequestExists),
		errors.Is(err, ErrJoinRequestNotPending),
		errors.Is(err, ErrGroupFull),
		errors.Is(err, ErrJoinRequestsLimit),
		errors.Is(err, ErrMessageDeleted):
		return http.StatusConflict
	case errors.Is(err, ErrInvalidUUID),
		errors.Is(err, ErrIncorrectData),
		errors.Is(err, ErrInvalidReply):
		return http.StatusBadRequest
	case errors.Is(err, ErrUserServiceError):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func Reason(err error) string {
	switch {
	case errors.Is(err, ErrBlockedByMe):
		return "BLOCKED_BY_ME"
	case errors.Is(err, ErrBlockedByThem):
		return "BLOCKED_BY_THEM"
	case errors.Is(err, ErrNotMember):
		return "NOT_MEMBER"
	case errors.Is(err, ErrInviteNotAllowed):
		return "GROUP_INVITE_NOT_ALLOWED"
	case errors.Is(err, ErrLastAdmin):
		return "LAST_ADMIN"
	case errors.Is(err, ErrBanned):
		return "USER_BANNED"
	case errors.Is(err, ErrGroupFull):
		return "GROUP_FULL"
	case errors.Is(err, ErrJoinRequestsLimit):
		return "JOIN_REQUESTS_LIMIT"
	case errors.Is(err, ErrJoinRequestExists):
		return "JOIN_REQUEST_EXISTS"
	case errors.Is(err, ErrJoinRequestNotPending):
		return "JOIN_REQUEST_NOT_PENDING"
	default:
		return ""
	}
}

func UserMessage(err error) string {
	switch {
	case errors.Is(err, ErrNotFound):
		return "Запись не найдена"
	case errors.Is(err, ErrUserNotFound):
		return "Пользователь не найден"
	case errors.Is(err, ErrForbidden):
		return "Недостаточно прав"
	case errors.Is(err, ErrNotMember):
		return "Вы не являетесь участником чата"
	case errors.Is(err, ErrBlockedByMe):
		return "Вы заблокировали этого пользователя"
	case errors.Is(err, ErrBlockedByThem):
		return "Этот пользователь заблокировал вас"
	case errors.Is(err, ErrUnauthorized):
		return "Необходима авторизация"
	case errors.Is(err, ErrInvalidUUID):
		return "Некорректный формат UUID"
	case errors.Is(err, ErrIncorrectData):
		return "Некорректные входные данные"
	case errors.Is(err, ErrInvalidReply):
		return "Нельзя ответить на сообщение из другого чата"
	case errors.Is(err, ErrMessageDeleted):
		return "Сообщение удалено"
	case errors.Is(err, ErrAlreadyMember):
		return "Пользователь уже состоит в чате"
	case errors.Is(err, ErrLastAdmin):
		return "Нельзя оставить группу без администратора"
	case errors.Is(err, ErrInviteNotAllowed):
		return "Пользователь запретил приглашать себя в группы"
	case errors.Is(err, ErrJoinRequestExists):
		return "Заявка на вступление уже отправлена"
	case errors.Is(err, ErrJoinRequestNotPending):
		return "Заявка уже обработана"
	case errors.Is(err, ErrBanned):
		return "Пользователь заблокирован в этой группе"
	case errors.Is(err, ErrGroupFull):
		return "Достигнут максимальный размер группы"
	case errors.Is(err, ErrJoinRequestsLimit):
		return "Слишком много заявок на рассмотрении"
	case errors.Is(err, ErrUserServiceError):
		return "Сервис временно недоступен"
	default:
		return "Внутренняя ошибка сервера"
	}
}
