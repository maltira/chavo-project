package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/proto/avatarshape"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
)

func TestGroupAvatarShape(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, member := uuid.New(), uuid.New()
	g := createGroup(t, e, admin, "public", member)
	id := g.Conversation.ID

	// права: только админ; посторонний - не участник; у direct фигуры нет
	if _, err := e.groups.SetAvatarShape(ctx, member, id, "circle", "#000000"); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("member: %v", err)
	}
	if _, err := e.groups.SetAvatarShape(ctx, uuid.New(), id, "circle", "#000000"); !errors.Is(err, apperror.ErrNotMember) {
		t.Fatalf("outsider: %v", err)
	}
	a, b := uuid.New(), uuid.New()
	m, _ := e.msgs.Send(ctx, a, service.SendMessageInput{RecipientID: &b, Content: "hi"})
	if _, err := e.groups.SetAvatarShape(ctx, a, m.ConversationID, "circle", "#000000"); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("direct: %v", err)
	}
	if _, err := e.groups.SetAvatarShape(ctx, admin, id, "hexagon", "#000000"); !errors.Is(err, avatarshape.ErrUnknownShape) {
		t.Fatalf("unknown shape: %v", err)
	}
	if _, err := e.groups.SetAvatarShape(ctx, admin, id, "circle", "orange"); !errors.Is(err, avatarshape.ErrInvalidColor) {
		t.Fatalf("bad color: %v", err)
	}
	sh, err := e.groups.SetAvatarShape(ctx, admin, id, "soft-boom", "#f59e0b")
	if err != nil || sh.Shape != "soft-boom" || sh.Color != "#F59E0B" {
		t.Fatalf("set: %+v %v", sh, err)
	}

	// фигура видна в карточке, в списке чатов и в поиске
	got, err := e.convs.Get(ctx, member, id)
	if err != nil || got.AvatarShape == nil || got.AvatarShape.Shape != "soft-boom" {
		t.Fatalf("get: %+v %v", got, err)
	}
	list, err := e.convs.List(ctx, member, 10, 0)
	if err != nil || len(list) != 1 || list[0].AvatarShape == nil || list[0].AvatarShape.Color != "#F59E0B" {
		t.Fatalf("list: %+v %v", list, err)
	}
	found, err := e.convs.SearchPublicGroups(ctx, uuid.New(), "Группа", 10, 0)
	if err != nil || len(found) != 1 || found[0].AvatarShape == nil || found[0].AvatarShape.Shape != "soft-boom" {
		t.Fatalf("search: %+v %v", found, err)
	}

	// новая фигура заменяет прежнюю целиком
	if _, err := e.groups.SetAvatarShape(ctx, admin, id, "bun", "#000000"); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.convs.Get(ctx, member, id); got.AvatarShape.Shape != "bun" || got.AvatarShape.Color != "#000000" {
		t.Fatalf("replace: %+v", got.AvatarShape)
	}

	// удаление: участнику нельзя, админу можно; повторное - не ошибка
	if err := e.groups.ClearAvatarShape(ctx, member, id); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("member clear: %v", err)
	}
	if err := e.groups.ClearAvatarShape(ctx, admin, id); err != nil {
		t.Fatal(err)
	}
	if err := e.groups.ClearAvatarShape(ctx, admin, id); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.convs.Get(ctx, member, id); got.AvatarShape != nil {
		t.Fatal("shape not cleared")
	}
	// только REST: событий нет
	if n := events(t, e, "conversation.updated"); n != 0 {
		t.Fatalf("shape changes published %d conversation.updated events", n)
	}

	// фигура удаляется вместе с группой
	if _, err := e.groups.SetAvatarShape(ctx, admin, id, "circle", "#FFFFFF"); err != nil {
		t.Fatal(err)
	}
	if err := e.groups.Delete(ctx, admin, id); err != nil {
		t.Fatal(err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM conversation_avatar_shapes WHERE conversation_id = $1`, id); n != 0 {
		t.Fatalf("shape left after group delete: %d", n)
	}
}

func TestPrivateGroupAvatarShapeInInvite(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin := uuid.New()
	g := createGroup(t, e, admin, "private")
	if _, err := e.groups.SetAvatarShape(ctx, admin, g.Conversation.ID, "ghost", "#312E81"); err != nil {
		t.Fatal(err)
	}
	info, err := e.joins.InviteInfo(ctx, uuid.New(), *g.InviteToken)
	if err != nil || info.AvatarShape == nil || info.AvatarShape.Shape != "ghost" {
		t.Fatalf("invite info: %+v %v", info, err)
	}
}
