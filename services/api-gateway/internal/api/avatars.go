package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	conversationv1 "github.com/maltira/chavo-project-backend/proto/gen/go/conversation/v1"
	userv1 "github.com/maltira/chavo-project-backend/proto/gen/go/user/v1"
	pmedia "github.com/maltira/chavo-project-backend/proto/media"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/httpx"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/media"
)

const (
	reasonAvatarInvalid     = "AVATAR_INVALID"
	reasonAvatarUnsupported = "AVATAR_TYPE_UNSUPPORTED"

	// частей multipart больше этого - запрос отклоняется, даже если поле file нашлось бы дальше
	maxUploadParts = 4
	// удаление старого или откат нового объекта не должно зависеть от отменённого запроса
	cleanupTimeout = 10 * time.Second
)

// readAvatar читает файл из поля file (multipart/form-data) и перекодирует его в аватарку.
// Имя файла и Content-Type части не используются: тип определяется по содержимому (media.ProcessAvatar).
// При ошибке ответ уже записан
func (h *Handler) readAvatar(c *gin.Context) ([]byte, bool) {
	mr, err := c.Request.MultipartReader()
	if err != nil {
		httpx.Abort(c, http.StatusBadRequest, "Ожидается файл в поле file (multipart/form-data)", httpx.ReasonInvalidRequest)
		return nil, false
	}
	var data []byte
	for parts := 0; data == nil; parts++ {
		if parts >= maxUploadParts {
			httpx.Abort(c, http.StatusBadRequest, "Слишком много полей в запросе", httpx.ReasonInvalidRequest)
			return nil, false
		}
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			httpx.Abort(c, http.StatusBadRequest, "Ожидается файл в поле file (multipart/form-data)", httpx.ReasonInvalidRequest)
			return nil, false
		}
		if err != nil {
			abortReadError(c, err)
			return nil, false
		}
		if part.FormName() != "file" {
			_, err = io.Copy(io.Discard, part)
			_ = part.Close()
			if err != nil {
				abortReadError(c, err)
				return nil, false
			}
			continue
		}
		data, err = io.ReadAll(io.LimitReader(part, media.MaxUploadBytes+1))
		_ = part.Close()
		if err != nil {
			abortReadError(c, err)
			return nil, false
		}
		if data == nil {
			data = []byte{}
		}
	}
	if len(data) > media.MaxUploadBytes {
		httpx.Abort(c, http.StatusRequestEntityTooLarge, "Слишком большой файл", httpx.ReasonTooLarge)
		return nil, false
	}

	avatar, err := media.ProcessAvatar(data)
	switch {
	case errors.Is(err, media.ErrUnsupportedType):
		httpx.Abort(c, http.StatusUnsupportedMediaType, "Можно загрузить только фото в формате JPEG, PNG, GIF или WebP", reasonAvatarUnsupported)
		return nil, false
	case errors.Is(err, media.ErrInvalidImage):
		httpx.Abort(c, http.StatusBadRequest, "Не удалось прочитать фото: файл повреждён, слишком маленький или слишком большой", reasonAvatarInvalid)
		return nil, false
	case err != nil:
		h.log.Error("Avatar encoding failed", zap.Error(err))
		httpx.Internal(c)
		return nil, false
	}
	// в лог - только итог и размеры, без содержимого и имени файла
	h.log.Debug("Avatar processed", zap.Int("upload_bytes", len(data)), zap.Int("avatar_bytes", len(avatar)))
	return avatar, true
}

func abortReadError(c *gin.Context, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		httpx.Abort(c, http.StatusRequestEntityTooLarge, "Слишком большой файл", httpx.ReasonTooLarge)
		return
	}
	httpx.Abort(c, http.StatusBadRequest, "Некорректный запрос с файлом", httpx.ReasonInvalidRequest)
}

// storeAvatar кладёт перекодированную аватарку в хранилище под новым случайным именем и возвращает её путь
func (h *Handler) storeAvatar(c *gin.Context, avatar []byte) (string, bool) {
	name, err := pmedia.NewAvatarName()
	if err != nil {
		h.log.Error("Avatar name generation failed", zap.Error(err))
		httpx.Internal(c)
		return "", false
	}
	if err = h.media.Put(c.Request.Context(), pmedia.AvatarKey(name), avatar, "image/jpeg"); err != nil {
		h.log.Error("Avatar upload to storage failed", zap.Error(err))
		httpx.Unavailable(c)
		return "", false
	}
	return pmedia.AvatarURL(name), true
}

// dropAvatar удаляет файл прежней (или не пригодившейся) аватарки; чужие и внешние URL не трогает.
// Ошибка не мешает ответу: в худшем случае в хранилище остаётся ненужный файл
func (h *Handler) dropAvatar(url string) {
	name, ok := pmedia.AvatarNameFromURL(url)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	if err := h.media.Delete(ctx, pmedia.AvatarKey(name)); err != nil {
		h.log.Warn("Avatar object left in storage", zap.String("key", pmedia.AvatarKey(name)), zap.Error(err))
	}
}

// uploadMyAvatar: POST /api/users/me/avatar - новое фото профиля; прежний файл удаляется
func (h *Handler) uploadMyAvatar(c *gin.Context) {
	avatar, ok := h.readAvatar(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	me, err := h.users.GetMe(ctx, &userv1.GetMeRequest{})
	if err != nil {
		h.fail(c, err)
		return
	}
	url, ok := h.storeAvatar(c, avatar)
	if !ok {
		return
	}
	if _, err = h.users.UpdateMe(ctx, &userv1.UpdateMeRequest{AvatarUrl: &url}); err != nil {
		h.dropAvatar(url)
		h.fail(c, err)
		return
	}
	h.dropAvatar(me.GetProfile().GetAvatarUrl())
	c.JSON(http.StatusOK, gin.H{"avatar_url": url})
}

// deleteMyAvatar: DELETE /api/users/me/avatar - убрать фото профиля
func (h *Handler) deleteMyAvatar(c *gin.Context) {
	ctx := c.Request.Context()
	me, err := h.users.GetMe(ctx, &userv1.GetMeRequest{})
	if err != nil {
		h.fail(c, err)
		return
	}
	empty := ""
	if _, err = h.users.UpdateMe(ctx, &userv1.UpdateMeRequest{AvatarUrl: &empty}); err != nil {
		h.fail(c, err)
		return
	}
	h.dropAvatar(me.GetProfile().GetAvatarUrl())
	httpx.Success(c, http.StatusOK, "Фото удалено")
}

// uploadGroupAvatar: POST /api/conversations/:id/avatar - новое фото группы (только админ;
// права проверяет conversation-service, при отказе загруженный файл удаляется)
func (h *Handler) uploadGroupAvatar(c *gin.Context) {
	avatar, ok := h.readAvatar(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	id := c.Param("id")
	conv, err := h.conversations.GetConversation(ctx, &conversationv1.GetConversationRequest{ConversationId: id})
	if err != nil {
		h.fail(c, err)
		return
	}
	url, ok := h.storeAvatar(c, avatar)
	if !ok {
		return
	}
	if _, err = h.conversations.UpdateGroup(ctx, &conversationv1.UpdateGroupRequest{ConversationId: id, AvatarUrl: &url}); err != nil {
		h.dropAvatar(url)
		h.fail(c, err)
		return
	}
	h.dropAvatar(conv.GetConversation().GetConversation().GetAvatarUrl())
	c.JSON(http.StatusOK, gin.H{"avatar_url": url})
}

// deleteGroupAvatar: DELETE /api/conversations/:id/avatar - убрать фото группы (только админ)
func (h *Handler) deleteGroupAvatar(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	conv, err := h.conversations.GetConversation(ctx, &conversationv1.GetConversationRequest{ConversationId: id})
	if err != nil {
		h.fail(c, err)
		return
	}
	empty := ""
	if _, err = h.conversations.UpdateGroup(ctx, &conversationv1.UpdateGroupRequest{ConversationId: id, AvatarUrl: &empty}); err != nil {
		h.fail(c, err)
		return
	}
	h.dropAvatar(conv.GetConversation().GetConversation().GetAvatarUrl())
	httpx.Success(c, http.StatusOK, "Фото удалено")
}

// serveAvatar: GET /media/avatars/:name - публичная отдача файла из приватного хранилища.
// Имя случайное (128 бит) и выдаётся только через API с учётом блокировок. Тип ставим сами, а заголовки
// запрещают браузеру исполнять ответ как страницу или скрипт, даже если объект оказался бы чужим
func (h *Handler) serveAvatar(c *gin.Context) {
	name := c.Param("name")
	if !pmedia.ValidAvatarName(name) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	body, err := h.media.Get(c.Request.Context(), pmedia.AvatarKey(name))
	if errors.Is(err, media.ErrNotFound) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if err != nil {
		h.log.Error("Avatar read from storage failed", zap.Error(err))
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	defer func() { _ = body.Close() }()

	hdr := c.Writer.Header()
	hdr.Set("Content-Type", "image/jpeg")
	hdr.Set("Cache-Control", "public, max-age=31536000, immutable")
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	hdr.Set("Cross-Origin-Resource-Policy", "same-origin")
	hdr.Set("Content-Disposition", `inline; filename="avatar.jpg"`)
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, io.LimitReader(body, media.MaxUploadBytes))
}
