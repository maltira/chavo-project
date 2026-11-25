package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
)

func pinEvents(t *testing.T, e *env, convID uuid.UUID) int {
	return count(t, e.pool, `SELECT COUNT(*) FROM outbox_events
		WHERE payload->>'event_type' = 'conversation.pins.changed' AND event_key = $1`, convID.String())
}

func pinIDs(t *testing.T, e *env, user, convID uuid.UUID) []string {
	t.Helper()
	pins, err := e.msgs.ListPins(context.Background(), user, convID)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(pins))
	for i, p := range pins {
		ids[i] = p.Message.ID.String()[:4] + ":" + p.Scope
	}
	return ids
}

func TestPinsInDirect(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	m1, _ := e.msgs.Send(ctx, a, service.SendMessageInput{RecipientID: &b, Content: "первое"})
	id := m1.ConversationID
	m2, _ := e.msgs.Send(ctx, b, service.SendMessageInput{ConversationID: &id, Content: "второе"})

	// для обоих - видят оба; только для себя - видит только закрепивший
	pin, err := e.msgs.PinMessage(ctx, a, id, m1.ID, models.PinScopeAll)
	if err != nil || pin.Scope != models.PinScopeAll || pin.PinnedBy != a || *pin.Message.Content != "первое" {
		t.Fatalf("pin all: %+v %v", pin, err)
	}
	if _, err = e.msgs.PinMessage(ctx, b, id, m2.ID, models.PinScopeMe); err != nil {
		t.Fatal(err)
	}
	if got := pinIDs(t, e, a, id); len(got) != 1 || got[0] != m1.ID.String()[:4]+":all" {
		t.Fatalf("a sees %v", got)
	}
	if got := pinIDs(t, e, b, id); len(got) != 2 || got[0] != m2.ID.String()[:4]+":me" {
		t.Fatalf("b sees %v (newest first)", got)
	}
	// одно сообщение можно закрепить и для всех, и для себя; повтор ничего не меняет
	if _, err = e.msgs.PinMessage(ctx, b, id, m1.ID, models.PinScopeMe); err != nil {
		t.Fatal(err)
	}
	before := pinEvents(t, e, id)
	if _, err = e.msgs.PinMessage(ctx, a, id, m1.ID, models.PinScopeAll); err != nil || pinEvents(t, e, id) != before {
		t.Fatalf("repeat pin: %v", err)
	}

	// событие: общий закреп - обоим, личный - только владельцу
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->>'event_type' = 'conversation.pins.changed'
		AND payload->'payload'->'member_ids' = to_jsonb(ARRAY[$1::text])`, b.String()); n != 2 {
		t.Fatalf("personal pin events to b = %d", n)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->>'event_type' = 'conversation.pins.changed'
		AND jsonb_array_length(payload->'payload'->'member_ids') = 2`); n != 1 {
		t.Fatalf("shared pin events = %d", n)
	}

	// снять общий закреп может любой из двоих, чужой личный - нет (у него свой личный список)
	if err = e.msgs.UnpinMessage(ctx, a, id, m2.ID, models.PinScopeMe); err != nil {
		t.Fatal(err)
	}
	if got := pinIDs(t, e, b, id); len(got) != 3 {
		t.Fatalf("a removed b's personal pin: %v", got)
	}
	if err = e.msgs.UnpinMessage(ctx, a, id, m1.ID, models.PinScopeAll); err != nil {
		t.Fatal(err)
	}
	if got := pinIDs(t, e, a, id); len(got) != 0 {
		t.Fatalf("a after unpin: %v", got)
	}

	// удаление сообщения снимает все его закрепы
	if err = e.msgs.Delete(ctx, b, m2.ID); err != nil {
		t.Fatal(err)
	}
	if got := pinIDs(t, e, b, id); len(got) != 1 || got[0] != m1.ID.String()[:4]+":me" {
		t.Fatalf("after delete: %v", got)
	}
	if _, err = e.msgs.PinMessage(ctx, b, id, m2.ID, models.PinScopeAll); !errors.Is(err, apperror.ErrMessageDeleted) {
		t.Fatalf("pin deleted: %v", err)
	}

	// неверный scope, чужой чат, посторонний
	if _, err = e.msgs.PinMessage(ctx, a, id, m1.ID, "everyone"); !errors.Is(err, apperror.ErrIncorrectData) {
		t.Fatalf("bad scope: %v", err)
	}
	if _, err = e.msgs.PinMessage(ctx, uuid.New(), id, m1.ID, models.PinScopeAll); !errors.Is(err, apperror.ErrNotMember) {
		t.Fatalf("outsider: %v", err)
	}
	other, _ := e.msgs.Send(ctx, a, service.SendMessageInput{RecipientID: ptr(uuid.New()), Content: "x"})
	if _, err = e.msgs.PinMessage(ctx, a, id, other.ID, models.PinScopeAll); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("message from another chat: %v", err)
	}
}

func TestPinsInGroup(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, member := uuid.New(), uuid.New()
	g := createGroup(t, e, admin, "private", member)
	id := g.Conversation.ID
	m, _ := e.msgs.Send(ctx, member, service.SendMessageInput{ConversationID: &id, Content: "важное"})

	if _, err := e.msgs.PinMessage(ctx, member, id, m.ID, models.PinScopeAll); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("member pins: %v", err)
	}
	if _, err := e.msgs.PinMessage(ctx, admin, id, m.ID, models.PinScopeMe); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("personal pin in group: %v", err)
	}
	if _, err := e.msgs.PinMessage(ctx, admin, id, m.ID, models.PinScopeAll); err != nil {
		t.Fatal(err)
	}
	if got := pinIDs(t, e, member, id); len(got) != 1 {
		t.Fatalf("member sees %v", got)
	}
	if err := e.msgs.UnpinMessage(ctx, member, id, m.ID, models.PinScopeAll); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("member unpins: %v", err)
	}

	// системные сообщения не закрепляются
	if err := e.groups.AddMembers(ctx, admin, id, []uuid.UUID{uuid.New()}); err != nil {
		t.Fatal(err)
	}
	var sys uuid.UUID
	if err := e.pool.QueryRow(ctx, `SELECT id FROM messages WHERE conversation_id = $1 AND kind = 'system' LIMIT 1`, id).Scan(&sys); err != nil {
		t.Fatal(err)
	}
	if _, err := e.msgs.PinMessage(ctx, admin, id, sys, models.PinScopeAll); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("system pin: %v", err)
	}

	// лимит общих закрепов
	for i := 0; i < 9; i++ {
		msg, _ := e.msgs.Send(ctx, admin, service.SendMessageInput{ConversationID: &id, Content: "m"})
		if _, err := e.msgs.PinMessage(ctx, admin, id, msg.ID, models.PinScopeAll); err != nil {
			t.Fatalf("pin %d: %v", i, err)
		}
	}
	extra, _ := e.msgs.Send(ctx, admin, service.SendMessageInput{ConversationID: &id, Content: "m"})
	if _, err := e.msgs.PinMessage(ctx, admin, id, extra.ID, models.PinScopeAll); !errors.Is(err, apperror.ErrMessagePinLimit) {
		t.Fatalf("limit: %v", err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM pinned_messages WHERE conversation_id = $1`, id); n != 10 {
		t.Fatalf("pins = %d", n)
	}
}

func TestPinsFollowHistoryVisibility(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	m, _ := e.msgs.Send(ctx, a, service.SendMessageInput{RecipientID: &b, Content: "старое"})
	id := m.ConversationID
	if _, err := e.msgs.PinMessage(ctx, a, id, m.ID, models.PinScopeAll); err != nil {
		t.Fatal(err)
	}
	// удалил чат у себя - старый закреп не виден, у собеседника остаётся
	if err := e.groups.Delete(ctx, b, id, false); err != nil {
		t.Fatal(err)
	}
	if got := pinIDs(t, e, b, id); len(got) != 0 {
		t.Fatalf("cleared chat shows pin: %v", got)
	}
	if got := pinIDs(t, e, a, id); len(got) != 1 {
		t.Fatalf("peer lost pin: %v", got)
	}
}

func TestUnpinInDirect(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	m, _ := e.msgs.Send(ctx, a, service.SendMessageInput{RecipientID: &b, Content: "общее"})
	id := m.ConversationID
	only := func(user uuid.UUID, want ...string) {
		t.Helper()
		got := pinIDs(t, e, user, id)
		if len(got) != len(want) {
			t.Fatalf("%v sees %v, want %v", user, got, want)
		}
		for i := range want {
			if got[i] != m.ID.String()[:4]+":"+want[i] {
				t.Fatalf("%v sees %v, want %v", user, got, want)
			}
		}
	}

	// общий закреп «только у меня»: у меня пропал, у собеседника остался его личным
	if _, err := e.msgs.PinMessage(ctx, a, id, m.ID, models.PinScopeAll); err != nil {
		t.Fatal(err)
	}
	if err := e.msgs.UnpinMessage(ctx, a, id, m.ID, models.PinScopeMe); err != nil {
		t.Fatal(err)
	}
	only(a)
	only(b, "me")

	// у собеседника уже был свой закреп - общий просто снимается, дубля нет
	if _, err := e.msgs.PinMessage(ctx, a, id, m.ID, models.PinScopeAll); err != nil {
		t.Fatal(err)
	}
	if err := e.msgs.UnpinMessage(ctx, a, id, m.ID, models.PinScopeMe); err != nil {
		t.Fatal(err)
	}
	only(a)
	only(b, "me")

	// «открепить у обоих» снимает общий и мой личный; личный собеседника - его
	if _, err := e.msgs.PinMessage(ctx, a, id, m.ID, models.PinScopeAll); err != nil {
		t.Fatal(err)
	}
	if _, err := e.msgs.PinMessage(ctx, a, id, m.ID, models.PinScopeMe); err != nil {
		t.Fatal(err)
	}
	if err := e.msgs.UnpinMessage(ctx, a, id, m.ID, models.PinScopeAll); err != nil {
		t.Fatal(err)
	}
	only(a)
	only(b, "me")
}

func TestPinnedSystemMessage(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	m, _ := e.msgs.Send(ctx, a, service.SendMessageInput{RecipientID: &b, Content: "встреча в 10"})
	id := m.ConversationID
	pinned := func() int {
		return count(t, e.pool, `SELECT COUNT(*) FROM messages WHERE conversation_id = $1
			AND kind = 'system' AND system_event = 'pinned' AND reply_to_message_id = $2`, id, m.ID)
	}

	// личный закреп собеседник не должен заметить - строки нет
	if _, err := e.msgs.PinMessage(ctx, a, id, m.ID, models.PinScopeMe); err != nil {
		t.Fatal(err)
	}
	if n := pinned(); n != 0 {
		t.Fatalf("personal pin wrote %d system messages", n)
	}

	// общий - одна строка (повтор не дублирует), она последняя в истории и поднимает чат, но не непрочитанное
	for i := 0; i < 2; i++ {
		if _, err := e.msgs.PinMessage(ctx, b, id, m.ID, models.PinScopeAll); err != nil {
			t.Fatal(err)
		}
	}
	if n := pinned(); n != 1 {
		t.Fatalf("shared pin wrote %d system messages", n)
	}
	list, _ := e.convs.List(ctx, a, false, 10, 0)
	last := list[0].LastMessage
	if last == nil || last.Kind != models.MessageSystem || last.SystemEvent == nil || *last.SystemEvent != models.SystemPinned ||
		last.SenderID != b || last.ReplyToMessageID == nil || *last.ReplyToMessageID != m.ID || last.TargetID != nil {
		t.Fatalf("last message: %+v", last)
	}
	if list[0].UnreadCount != 0 {
		t.Fatalf("system message counted as unread: %d", list[0].UnreadCount)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->>'event_type' = 'message.created'
		AND payload->'payload'->>'system_event' = 'pinned' AND payload->'payload'->>'reply_to_message_id' = $1`, m.ID.String()); n != 1 {
		t.Fatalf("message.created for pin = %d", n)
	}

	// открепление строки не пишет
	if err := e.msgs.UnpinMessage(ctx, b, id, m.ID, models.PinScopeAll); err != nil {
		t.Fatal(err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM messages WHERE conversation_id = $1 AND kind = 'system'`, id); n != 1 {
		t.Fatalf("system messages after unpin = %d", n)
	}
}

func ptr[T any](v T) *T { return &v }
