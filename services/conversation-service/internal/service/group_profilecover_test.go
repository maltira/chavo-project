package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/proto/profilecover"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
)

func TestGroupProfileCover(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, member := uuid.New(), uuid.New()
	g := createGroup(t, e, admin, "public", member)
	id := g.Conversation.ID

	// права: только админ; посторонний - не участник; у direct обложки нет
	if _, err := e.groups.SetProfileCover(ctx, member, id, "#4F46E5", "star"); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("member: %v", err)
	}
	if _, err := e.groups.SetProfileCover(ctx, uuid.New(), id, "#4F46E5", "star"); !errors.Is(err, apperror.ErrNotMember) {
		t.Fatalf("outsider: %v", err)
	}
	a, b := uuid.New(), uuid.New()
	m, _ := e.msgs.Send(ctx, a, service.SendMessageInput{RecipientID: &b, Content: "hi"})
	if _, err := e.groups.SetProfileCover(ctx, a, m.ConversationID, "#4F46E5", "star"); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("direct: %v", err)
	}
	if _, err := e.groups.SetProfileCover(ctx, admin, id, "#4F46E5", "unknown"); !errors.Is(err, profilecover.ErrUnknownPattern) {
		t.Fatalf("unknown pattern: %v", err)
	}
	if _, err := e.groups.SetProfileCover(ctx, admin, id, "orange", "star"); !errors.Is(err, profilecover.ErrInvalidColor) {
		t.Fatalf("bad color: %v", err)
	}
	cov, err := e.groups.SetProfileCover(ctx, admin, id, "#f59e0b", "star")
	if err != nil || cov.Color != "#F59E0B" || cov.Pattern != "star" {
		t.Fatalf("set: %+v %v", cov, err)
	}

	// обложка видна в карточке, в списке чатов и в поиске
	got, err := e.convs.Get(ctx, member, id)
	if err != nil || got.ProfileCover == nil || got.ProfileCover.Color != "#F59E0B" || got.ProfileCover.Pattern != "star" {
		t.Fatalf("get: %+v %v", got, err)
	}
	list, err := e.convs.List(ctx, member, false, 10, 0)
	if err != nil || len(list) != 1 || list[0].ProfileCover == nil || list[0].ProfileCover.Color != "#F59E0B" {
		t.Fatalf("list: %+v %v", list, err)
	}
	found, err := e.convs.SearchPublicGroups(ctx, uuid.New(), "Группа", 10, 0)
	if err != nil || len(found) != 1 || found[0].ProfileCover == nil || found[0].ProfileCover.Pattern != "star" {
		t.Fatalf("search: %+v %v", found, err)
	}

	// новая обложка без паттерна (только градиент) заменяет прежнюю целиком
	if _, err := e.groups.SetProfileCover(ctx, admin, id, "#4f46e5", ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.convs.Get(ctx, member, id); got.ProfileCover.Color != "#4F46E5" || got.ProfileCover.Pattern != "" {
		t.Fatalf("replace: %+v", got.ProfileCover)
	}

	// удаление: участнику нельзя, админу можно; повторное - не ошибка
	if err := e.groups.ClearProfileCover(ctx, member, id); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("member clear: %v", err)
	}
	if err := e.groups.ClearProfileCover(ctx, admin, id); err != nil {
		t.Fatal(err)
	}
	if err := e.groups.ClearProfileCover(ctx, admin, id); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.convs.Get(ctx, member, id); got.ProfileCover != nil {
		t.Fatal("cover not cleared")
	}
	// только REST: событий нет
	if n := events(t, e, "conversation.updated"); n != 0 {
		t.Fatalf("cover changes published %d conversation.updated events", n)
	}

	// обложка удаляется вместе с группой
	if _, err := e.groups.SetProfileCover(ctx, admin, id, "#FFFFFF", "moon"); err != nil {
		t.Fatal(err)
	}
	if err := e.groups.Delete(ctx, admin, id, false); err != nil {
		t.Fatal(err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM conversation_profile_covers WHERE conversation_id = $1`, id); n != 0 {
		t.Fatalf("cover left after group delete: %d", n)
	}
}

func TestPrivateGroupProfileCoverInInvite(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin := uuid.New()
	g := createGroup(t, e, admin, "private")
	if _, err := e.groups.SetProfileCover(ctx, admin, g.Conversation.ID, "#312E81", "moon"); err != nil {
		t.Fatal(err)
	}
	info, err := e.joins.InviteInfo(ctx, uuid.New(), *g.InviteToken)
	if err != nil || info.ProfileCover == nil || info.ProfileCover.Color != "#312E81" || info.ProfileCover.Pattern != "moon" {
		t.Fatalf("invite info: %+v %v", info, err)
	}
}
