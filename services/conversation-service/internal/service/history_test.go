package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
)

func sendTo(t *testing.T, e *env, from, conv uuid.UUID, text string, reply *uuid.UUID) uuid.UUID {
	t.Helper()
	m, err := e.msgs.Send(context.Background(), from, service.SendMessageInput{ConversationID: &conv, Content: text, ReplyToMessageID: reply})
	if err != nil {
		t.Fatalf("send %q: %v", text, err)
	}
	return m.ID
}

func TestNewMemberSeesHistoryOnlyFromJoin(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, m1, c := uuid.New(), uuid.New(), uuid.New()
	conv := createGroup(t, e, admin, "", m1).Conversation.ID

	old1 := sendTo(t, e, admin, conv, "старое 1", nil)
	sendTo(t, e, m1, conv, "старое 2", nil)
	if err := e.groups.AddMembers(ctx, admin, conv, []uuid.UUID{c}); err != nil {
		t.Fatal(err)
	}

	list, err := e.msgs.List(ctx, c, conv, nil, 50)
	if err != nil || len(list) != 0 {
		t.Fatalf("new member list = %d, %v; want empty", len(list), err)
	}
	sum, err := e.convs.Get(ctx, c, conv)
	if err != nil || sum.UnreadCount != 0 || sum.LastMessage != nil {
		t.Fatalf("summary before new messages: %+v %v", sum, err)
	}

	fresh := sendTo(t, e, admin, conv, "новое", nil)
	list, _ = e.msgs.List(ctx, c, conv, nil, 50)
	if len(list) != 1 || list[0].ID != fresh {
		t.Fatalf("list after join = %+v", list)
	}
	sum, _ = e.convs.Get(ctx, c, conv)
	if sum.UnreadCount != 1 || sum.LastMessage == nil || sum.LastMessage.ID != fresh {
		t.Fatalf("summary after new message: %+v", sum)
	}

	// До joined_at: нельзя ни пометить прочитанным, ни смотреть читателей, ни листать от этого курсора.
	if err = e.msgs.MarkRead(ctx, c, conv, old1); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("mark read hidden: %v", err)
	}
	if _, err = e.msgs.Readers(ctx, c, conv, old1); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("readers hidden: %v", err)
	}
	if _, err = e.msgs.List(ctx, c, conv, &old1, 10); !errors.Is(err, apperror.ErrIncorrectData) {
		t.Fatalf("before hidden: %v", err)
	}
	// У старожилов история целая.
	if list, _ = e.msgs.List(ctx, m1, conv, nil, 50); len(list) != 3 {
		t.Fatalf("old member list = %d, want 3", len(list))
	}
}

func TestAdminSeesFullHistory(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, c := uuid.New(), uuid.New()
	conv := createGroup(t, e, admin, "").Conversation.ID

	sendTo(t, e, admin, conv, "раз", nil)
	last := sendTo(t, e, admin, conv, "два", nil)
	if err := e.groups.AddMembers(ctx, admin, conv, []uuid.UUID{c}); err != nil {
		t.Fatal(err)
	}
	if list, _ := e.msgs.List(ctx, c, conv, nil, 50); len(list) != 0 {
		t.Fatalf("member sees %d old messages", len(list))
	}

	if err := e.groups.SetRole(ctx, admin, conv, c, "admin"); err != nil {
		t.Fatal(err)
	}
	if list, _ := e.msgs.List(ctx, c, conv, nil, 50); len(list) != 2 {
		t.Fatalf("admin sees %d messages, want 2", len(list))
	}
	sum, _ := e.convs.Get(ctx, c, conv)
	if sum.UnreadCount != 0 || sum.LastMessage == nil || sum.LastMessage.ID != last {
		t.Fatalf("admin summary: unread=%d last=%+v (unread counts only from joined_at)", sum.UnreadCount, sum.LastMessage)
	}

	if err := e.groups.SetRole(ctx, admin, conv, c, "member"); err != nil {
		t.Fatal(err)
	}
	if list, _ := e.msgs.List(ctx, c, conv, nil, 50); len(list) != 0 {
		t.Fatalf("demoted member sees %d old messages", len(list))
	}
}

func TestLeftMemberLosesChatAndRejoinStartsNewHistory(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, m1 := uuid.New(), uuid.New()
	conv := createGroup(t, e, admin, "public", m1).Conversation.ID

	sendTo(t, e, admin, conv, "A", nil)
	byM1 := sendTo(t, e, m1, conv, "B от m1", nil)
	if err := e.groups.RemoveMember(ctx, m1, conv, m1, false); err != nil {
		t.Fatal(err)
	}

	if _, err := e.msgs.List(ctx, m1, conv, nil, 50); !errors.Is(err, apperror.ErrNotMember) {
		t.Fatalf("left member list: %v", err)
	}
	if chats, _ := e.convs.List(ctx, m1, 10, 0); len(chats) != 0 {
		t.Fatalf("left member still sees %d chats", len(chats))
	}
	// Сообщения ушедшего остаются у остальных.
	adminList, _ := e.msgs.List(ctx, admin, conv, nil, 50)
	found := false
	for _, m := range adminList {
		found = found || m.ID == byM1
	}
	if !found {
		t.Fatal("messages of a left member must stay for others")
	}

	sendTo(t, e, admin, conv, "C пока его нет", nil)
	if err := e.groups.Join(ctx, m1, conv); err != nil {
		t.Fatal(err)
	}
	if list, _ := e.msgs.List(ctx, m1, conv, nil, 50); len(list) != 0 {
		t.Fatalf("rejoined member sees %d old messages, want 0", len(list))
	}
	fresh := sendTo(t, e, admin, conv, "D после возвращения", nil)
	if list, _ := e.msgs.List(ctx, m1, conv, nil, 50); len(list) != 1 || list[0].ID != fresh {
		t.Fatalf("rejoined list = %+v", list)
	}
}

func TestReplyTargetBeforeJoinIsReadableAlone(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, c, outsider := uuid.New(), uuid.New(), uuid.New()
	conv := createGroup(t, e, admin, "").Conversation.ID

	other := sendTo(t, e, admin, conv, "просто старое", nil)
	target := sendTo(t, e, admin, conv, "цель reply", nil)
	if err := e.groups.AddMembers(ctx, admin, conv, []uuid.UUID{c}); err != nil {
		t.Fatal(err)
	}
	sendTo(t, e, admin, conv, "ответ", &target)

	got, err := e.msgs.Get(ctx, c, target)
	if err != nil || got.Content == nil || *got.Content != "цель reply" {
		t.Fatalf("reply target: %+v %v", got, err)
	}
	if _, err = e.msgs.Get(ctx, c, other); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("unrelated old message: %v", err)
	}
	if _, err = e.msgs.Get(ctx, outsider, target); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("outsider: %v", err)
	}
	if list, _ := e.msgs.List(ctx, c, conv, nil, 50); len(list) != 1 {
		t.Fatalf("list must still hide history, got %d", len(list))
	}
}

func TestBanPublicGroup(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, m1, m2, plain := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	conv := createGroup(t, e, admin, "public", m1, m2, plain).Conversation.ID

	if err := e.groups.RemoveMember(ctx, admin, conv, admin, true); !errors.Is(err, apperror.ErrIncorrectData) {
		t.Fatalf("ban self: %v", err)
	}
	if err := e.groups.RemoveMember(ctx, m2, conv, m1, true); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("ban by member: %v", err)
	}
	if err := e.groups.RemoveMember(ctx, admin, conv, m1, true); err != nil {
		t.Fatal(err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->>'event_type' = 'conversation.member.removed' AND payload->'payload'->>'banned' = 'true'`); n != 1 {
		t.Fatalf("banned member.removed events = %d, want 1", n)
	}

	if err := e.groups.Join(ctx, m1, conv); !errors.Is(err, apperror.ErrBanned) {
		t.Fatalf("banned join: %v", err)
	}
	if err := e.groups.AddMembers(ctx, admin, conv, []uuid.UUID{m1}); !errors.Is(err, apperror.ErrBanned) {
		t.Fatalf("banned add: %v", err)
	}
	if _, err := e.groups.ListBans(ctx, m2, conv, 50, 0); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("list bans by member: %v", err)
	}
	bans, err := e.groups.ListBans(ctx, admin, conv, 50, 0)
	if err != nil || len(bans) != 1 || bans[0].UserID != m1 || bans[0].BannedBy != admin {
		t.Fatalf("bans = %+v %v", bans, err)
	}

	// Обычное исключение возвращению не мешает.
	if err = e.groups.RemoveMember(ctx, admin, conv, plain, false); err != nil {
		t.Fatal(err)
	}
	if err = e.groups.Join(ctx, plain, conv); err != nil {
		t.Fatalf("kicked without ban must be able to rejoin: %v", err)
	}

	if err = e.groups.Unban(ctx, m2, conv, m1); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("unban by member: %v", err)
	}
	if err = e.groups.Unban(ctx, admin, conv, m1); err != nil {
		t.Fatal(err)
	}
	if err = e.groups.Unban(ctx, admin, conv, m1); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("repeat unban: %v", err)
	}
	if err = e.groups.Join(ctx, m1, conv); err != nil {
		t.Fatalf("join after unban: %v", err)
	}
}

func TestBanPrivateGroupBlocksJoinRequests(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, m1 := uuid.New(), uuid.New()
	res := createGroup(t, e, admin, "", m1)
	conv, token := res.Conversation.ID, *res.InviteToken

	if err := e.groups.RemoveMember(ctx, admin, conv, m1, true); err != nil {
		t.Fatal(err)
	}
	if _, err := e.joins.RequestJoin(ctx, m1, conv, token); !errors.Is(err, apperror.ErrBanned) {
		t.Fatalf("banned request: %v", err)
	}
	if err := e.groups.Unban(ctx, admin, conv, m1); err != nil {
		t.Fatal(err)
	}
	if _, err := e.joins.RequestJoin(ctx, m1, conv, token); err != nil {
		t.Fatalf("request after unban: %v", err)
	}
}
