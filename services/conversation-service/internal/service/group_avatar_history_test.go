package service_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/proto/media"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
)

func groupPhoto(n int) string { return media.AvatarURL(fmt.Sprintf("%032x.jpg", n)) }

func photoURLs(t *testing.T, e *env, user, group uuid.UUID) []string {
	t.Helper()
	list, err := e.groups.ListAvatars(context.Background(), user, group)
	if err != nil {
		t.Fatal(err)
	}
	urls := make([]string, len(list))
	for i, p := range list {
		urls[i] = p.URL
	}
	return urls
}

func groupAvatarURL(t *testing.T, e *env, group uuid.UUID) *string {
	t.Helper()
	var url *string
	if err := e.pool.QueryRow(context.Background(), `SELECT avatar_url FROM conversations WHERE id = $1`, group).Scan(&url); err != nil {
		t.Fatal(err)
	}
	return url
}

func updatedEvents(t *testing.T, e *env, group uuid.UUID) int {
	return count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE event_key = $1
		AND payload->>'event_type' = 'conversation.updated' AND payload->'payload'->'fields' = '["avatar_url"]'::jsonb`, group.String())
}

func TestGroupAvatarHistory(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, member := uuid.New(), uuid.New()
	g := createGroup(t, e, admin, "private", member)
	id := g.Conversation.ID

	// права: только админ; посторонний - не участник
	if _, err := e.groups.AddAvatar(ctx, member, id, groupPhoto(1)); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("member adds: %v", err)
	}
	if _, err := e.groups.AddAvatar(ctx, uuid.New(), id, groupPhoto(1)); !errors.Is(err, apperror.ErrNotMember) {
		t.Fatalf("outsider adds: %v", err)
	}
	if _, err := e.groups.AddAvatar(ctx, admin, id, "https://example.com/a.png"); !errors.Is(err, media.ErrInvalidAvatarURL) {
		t.Fatalf("foreign url: %v", err)
	}
	if _, err := e.groups.AddAvatar(ctx, admin, id, ""); !errors.Is(err, apperror.ErrIncorrectData) {
		t.Fatalf("empty url: %v", err)
	}

	for n := 1; n <= 3; n++ {
		if evicted, err := e.groups.AddAvatar(ctx, admin, id, groupPhoto(n)); err != nil || len(evicted) != 0 {
			t.Fatalf("add %d: %v %v", n, evicted, err)
		}
	}
	if got := photoURLs(t, e, member, id); !slices.Equal(got, []string{groupPhoto(3), groupPhoto(2), groupPhoto(1)}) {
		t.Fatalf("member sees %v", got)
	}
	if cur := groupAvatarURL(t, e, id); cur == nil || *cur != groupPhoto(3) {
		t.Fatalf("group avatar: %v", cur)
	}
	// каждое изменение - событие участникам
	if n := updatedEvents(t, e, id); n != 3 {
		t.Fatalf("conversation.updated events = %d", n)
	}

	// посторонний приватной группы историю не видит
	if _, err := e.groups.ListAvatars(ctx, uuid.New(), id); !errors.Is(err, apperror.ErrNotMember) {
		t.Fatalf("outsider list: %v", err)
	}

	// выбрать главной - старое фото наверху, группа следует за ним; чужое/неизвестное - ErrNotFound
	list, _ := e.groups.ListAvatars(ctx, admin, id)
	if err := e.groups.SelectAvatar(ctx, member, id, list[2].ID); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("member selects: %v", err)
	}
	if err := e.groups.SelectAvatar(ctx, admin, id, list[2].ID); err != nil {
		t.Fatal(err)
	}
	if got := photoURLs(t, e, admin, id); !slices.Equal(got, []string{groupPhoto(1), groupPhoto(3), groupPhoto(2)}) {
		t.Fatalf("after select: %v", got)
	}
	if cur := groupAvatarURL(t, e, id); *cur != groupPhoto(1) {
		t.Fatalf("group avatar after select: %v", *cur)
	}
	if err := e.groups.SelectAvatar(ctx, admin, id, uuid.New()); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("select unknown: %v", err)
	}

	// лимит: шестое фото вытесняет самое старое
	for n := 4; n <= 5; n++ {
		if _, err := e.groups.AddAvatar(ctx, admin, id, groupPhoto(n)); err != nil {
			t.Fatal(err)
		}
	}
	evicted, err := e.groups.AddAvatar(ctx, admin, id, groupPhoto(6))
	if err != nil || !slices.Equal(evicted, []string{groupPhoto(2)}) {
		t.Fatalf("evicted: %v %v", evicted, err)
	}
	if got := photoURLs(t, e, admin, id); len(got) != 5 || got[0] != groupPhoto(6) {
		t.Fatalf("after limit: %v", got)
	}

	// удалить текущее: предыдущее становится текущим; удалить любое по id; затем всё - аватарки нет
	removed, err := e.groups.DeleteAvatar(ctx, admin, id, nil)
	if err != nil || removed != groupPhoto(6) {
		t.Fatalf("delete current: %q %v", removed, err)
	}
	if cur := groupAvatarURL(t, e, id); *cur != groupPhoto(5) {
		t.Fatalf("group avatar after delete: %v", *cur)
	}
	if _, err = e.groups.DeleteAvatar(ctx, member, id, nil); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("member deletes: %v", err)
	}
	list, _ = e.groups.ListAvatars(ctx, admin, id)
	if removed, err = e.groups.DeleteAvatar(ctx, admin, id, &list[1].ID); err != nil || removed != list[1].URL {
		t.Fatalf("delete by id: %q %v", removed, err)
	}
	for {
		if _, err = e.groups.DeleteAvatar(ctx, admin, id, nil); errors.Is(err, apperror.ErrNotFound) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
	}
	if cur := groupAvatarURL(t, e, id); cur != nil {
		t.Fatalf("group avatar must be NULL: %v", *cur)
	}
}

func TestGroupAvatarHistoryVisibility(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, stranger := uuid.New(), uuid.New()
	pub := createGroup(t, e, admin, "public").Conversation.ID
	if _, err := e.groups.AddAvatar(ctx, admin, pub, groupPhoto(1)); err != nil {
		t.Fatal(err)
	}
	// открытая группа: история видна и не участнику (как и фото в поиске)
	if got := photoURLs(t, e, stranger, pub); !slices.Equal(got, []string{groupPhoto(1)}) {
		t.Fatalf("stranger in a public group sees %v", got)
	}
	// личный чат: истории аватарок нет
	a, b := uuid.New(), uuid.New()
	m, _ := e.msgs.Send(ctx, a, service.SendMessageInput{RecipientID: &b, Content: "hi"})
	if _, err := e.groups.ListAvatars(ctx, a, m.ConversationID); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("direct: %v", err)
	}
	if _, err := e.groups.AddAvatar(ctx, a, m.ConversationID, groupPhoto(2)); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("direct add: %v", err)
	}
}

func TestGroupAvatarHistoryDeletedWithGroup(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin := uuid.New()
	id := createGroup(t, e, admin, "private").Conversation.ID
	if _, err := e.groups.AddAvatar(ctx, admin, id, groupPhoto(9)); err != nil {
		t.Fatal(err)
	}
	if err := e.groups.Delete(ctx, admin, id, false); err != nil {
		t.Fatal(err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM conversation_avatars WHERE conversation_id = $1`, id); n != 0 {
		t.Fatalf("history rows after group delete: %d", n)
	}
}
