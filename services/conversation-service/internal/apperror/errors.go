package apperror

import (
	"errors"
	"net/http"

	"github.com/maltira/chavo-project-backend/proto/avatarshape"
	"github.com/maltira/chavo-project-backend/proto/grpcx"
	"github.com/maltira/chavo-project-backend/proto/media"
	"github.com/maltira/chavo-project-backend/proto/status"
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
	ErrSlugInvalid           = errors.New("invalid chat link code")
	ErrSlugRequired          = errors.New("public group requires a chat link code")
	ErrSlugTaken             = errors.New("chat link code is taken")
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
		errors.Is(err, ErrSlugTaken),
		errors.Is(err, ErrMessageDeleted):
		return http.StatusConflict
	case errors.Is(err, ErrInvalidUUID),
		errors.Is(err, ErrIncorrectData),
		errors.Is(err, ErrInvalidReply),
		errors.Is(err, ErrSlugInvalid),
		errors.Is(err, ErrSlugRequired),
		errors.Is(err, status.ErrUnknownIcon),
		errors.Is(err, status.ErrUnknownDuration),
		errors.Is(err, status.ErrInvalidText),
		errors.Is(err, status.ErrInvalidColor),
		errors.Is(err, avatarshape.ErrUnknownShape),
		errors.Is(err, avatarshape.ErrInvalidColor),
		errors.Is(err, media.ErrInvalidAvatarURL):
		return http.StatusBadRequest
	case errors.Is(err, ErrUserServiceError):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func Reason(err error) string {
	switch {
	case errors.Is(err, avatarshape.ErrUnknownShape):
		return "AVATAR_SHAPE_INVALID"
	case errors.Is(err, avatarshape.ErrInvalidColor):
		return "AVATAR_SHAPE_COLOR_INVALID"
	case errors.Is(err, media.ErrInvalidAvatarURL):
		return "AVATAR_INVALID"
	case errors.Is(err, status.ErrUnknownIcon):
		return "STATUS_ICON_INVALID"
	case errors.Is(err, status.ErrInvalidColor):
		return "STATUS_COLOR_INVALID"
	case errors.Is(err, status.ErrUnknownDuration):
		return "STATUS_DURATION_INVALID"
	case errors.Is(err, status.ErrInvalidText):
		return "STATUS_TEXT_INVALID"
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
	case errors.Is(err, ErrSlugInvalid):
		return "SLUG_INVALID"
	case errors.Is(err, ErrSlugRequired):
		return "SLUG_REQUIRED"
	case errors.Is(err, ErrSlugTaken):
		return "SLUG_TAKEN"
	default:
		return ""
	}
}

func UserMessage(err error) string {
	switch {
	case errors.Is(err, avatarshape.ErrUnknownShape):
		return "Неизвестная фигура"
	case errors.Is(err, avatarshape.ErrInvalidColor):
		return "Цвет фигуры задаётся в формате #RRGGBB"
	case errors.Is(err, media.ErrInvalidAvatarURL):
		return "Фото группы можно только загрузить"
	case errors.Is(err, status.ErrUnknownIcon):
		return "Неизвестная иконка статуса"
	case errors.Is(err, status.ErrInvalidColor):
		return "Цвета статуса задаются в формате #RRGGBB"
	case errors.Is(err, status.ErrUnknownDuration):
		return "Неизвестный срок статуса"
	case errors.Is(err, status.ErrInvalidText):
		return "Текст статуса — от 1 до 70 символов в одну строку"
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
	case errors.Is(err, ErrSlugInvalid):
		return "Ссылка на чат - от 3 до 32 латинских букв, цифр и _"
	case errors.Is(err, ErrSlugRequired):
		return "У открытой группы должна быть ссылка на чат"
	case errors.Is(err, ErrSlugTaken):
		return "Эта ссылка на чат уже занята"
	case errors.Is(err, ErrUserServiceError):
		return "Сервис временно недоступен"
	default:
		return "Внутренняя ошибка сервера"
	}
}

// GRPCStatus переводит ошибку в gRPC status по тем же таблицам, что и HTTP-коды; текст внутренних ошибок не раскрывается.
func GRPCStatus(err error) error {
	return grpcx.Error(grpcx.CodeFromHTTP(HTTPCode(err)), Reason(err), UserMessage(err))
}
