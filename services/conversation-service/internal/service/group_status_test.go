package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/proto/status"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
)

// in — статус для теста: иконка, текст и срок, цвета фиксированные.
func in(icon, text, duration string) status.Input {
	return status.Input{Icon: icon, Text: text, Background: "#F59E0B", Foreground: "#451A03", Duration: duration}
}

func TestGroupStatus(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, member := uuid.New(), uuid.New()
	g := createGroup(t, e, admin, "public", member)
	id := g.Conversation.ID

	// Права: только админ; посторонний — не участник; у direct статуса нет.
	if _, err := e.groups.SetStatus(ctx, member, id, in("heart", "x", "1h")); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("member: %v", err)
	}
	if _, err := e.groups.SetStatus(ctx, uuid.New(), id, in("heart", "x", "1h")); !errors.Is(err, apperror.ErrNotMember) {
		t.Fatalf("outsider: %v", err)
	}
	a, b := uuid.New(), uuid.New()
	m, _ := e.msgs.Send(ctx, a, service.SendMessageInput{RecipientID: &b, Content: "hi"})
	if _, err := e.groups.SetStatus(ctx, a, m.ConversationID, in("heart", "x", "1h")); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("direct: %v", err)
	}
	if _, err := e.groups.SetStatus(ctx, admin, id, in("heart", strings.Repeat("я", 71), "1h")); !errors.Is(err, status.ErrInvalidText) {
		t.Fatalf("long text: %v", err)
	}
	badColor := in("heart", "x", "1h")
	badColor.Background = "orange"
	if _, err := e.groups.SetStatus(ctx, admin, id, badColor); !errors.Is(err, status.ErrInvalidColor) {
		t.Fatalf("bad color: %v", err)
	}
	st, err := e.groups.SetStatus(ctx, admin, id, in("shine", " Сегодня отдыхаем ", "24h"))
	if err != nil || st.Icon != "shine" || st.Background != "#F59E0B" || st.Foreground != "#451A03" || st.Text != "Сегодня отдыхаем" || st.ExpiresAt == nil {
		t.Fatalf("set: %+v %v", st, err)
	}

	// Статус виден участникам в списке чатов и карточке, в поиске и в превью приглашения.
	got, err := e.convs.Get(ctx, member, id)
	if err != nil || got.Status == nil || got.Status.Text != "Сегодня отдыхаем" {
		t.Fatalf("get: %+v %v", got, err)
	}
	found, err := e.convs.SearchPublicGroups(ctx, uuid.New(), "Группа", 10, 0)
	if err != nil || len(found) != 1 || found[0].Status == nil || found[0].Status.Icon != "shine" {
		t.Fatalf("search: %+v %v", found, err)
	}

	// Новый статус заменяет прежний целиком.
	if _, err := e.groups.SetStatus(ctx, admin, id, in("moon", "Ночь", "never")); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.convs.Get(ctx, member, id); got.Status.Icon != "moon" || got.Status.ExpiresAt != nil {
		t.Fatalf("replace: %+v", got.Status)
	}

	// Истёкший статус не читается.
	if _, err := e.pool.Exec(ctx, `UPDATE conversation_statuses SET expires_at = NOW() - interval '1 minute' WHERE conversation_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.convs.Get(ctx, member, id); got.Status != nil {
		t.Fatalf("expired status read: %+v", got.Status)
	}

	// удаление: участнику нельзя, админу можно; повторное - не ошибка
	if err := e.groups.ClearStatus(ctx, member, id); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("member clear: %v", err)
	}
	if _, err := e.groups.SetStatus(ctx, admin, id, in("heart", "снова", "1h")); err != nil {
		t.Fatal(err)
	}
	if err := e.groups.ClearStatus(ctx, admin, id); err != nil {
		t.Fatal(err)
	}
	if err := e.groups.ClearStatus(ctx, admin, id); err != nil {
		t.Fatal(err)
	}
	// статус только через REST: ни установка, ни удаление событий не публикуют
	if n := events(t, e, "conversation.updated"); n != 0 {
		t.Fatalf("status changes published %d conversation.updated events", n)
	}
	if got, _ := e.convs.Get(ctx, member, id); got.Status != nil {
		t.Fatal("status not cleared")
	}

	// Статус удаляется вместе с группой.
	if _, err := e.groups.SetStatus(ctx, admin, id, in("heart", "до удаления", "1h")); err != nil {
		t.Fatal(err)
	}
	if err := e.groups.Delete(ctx, admin, id); err != nil {
		t.Fatal(err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM conversation_statuses WHERE conversation_id = $1`, id); n != 0 {
		t.Fatalf("status left after group delete: %d", n)
	}
}

func TestPrivateGroupStatusInInvite(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin := uuid.New()
	g := createGroup(t, e, admin, "private")
	if _, err := e.groups.SetStatus(ctx, admin, g.Conversation.ID, in("ok", "Читаем правила", "7d")); err != nil {
		t.Fatal(err)
	}
	info, err := e.joins.InviteInfo(ctx, uuid.New(), *g.InviteToken)
	if err != nil || info.Status == nil || info.Status.Text != "Читаем правила" {
		t.Fatalf("invite info: %+v %v", info, err)
	}
}
