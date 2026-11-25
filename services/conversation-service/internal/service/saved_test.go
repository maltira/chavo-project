package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
)

func TestSavedMessages(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	me := uuid.New()
	// user-service о себе не спрашиваем: даже «пропавший» пользователь пишет себе
	e.users.missing[me] = true

	first, err := e.msgs.Send(ctx, me, service.SendMessageInput{RecipientID: &me, Content: "заметка"})
	if err != nil {
		t.Fatal(err)
	}
	id := first.ConversationID
	// повторная отправка «себе» попадает в тот же чат
	second, err := e.msgs.Send(ctx, me, service.SendMessageInput{RecipientID: &me, Content: "ещё"})
	if err != nil || second.ConversationID != id {
		t.Fatalf("second: %+v %v", second, err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM conversation_members WHERE conversation_id = $1`, id); n != 1 {
		t.Fatalf("members = %d", n)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->>'event_type' = 'conversation.created'
		AND payload->'payload'->'member_ids' = to_jsonb(ARRAY[$1::text])`, me.String()); n != 1 {
		t.Fatalf("conversation.created = %d", n)
	}

	list, err := e.convs.List(ctx, me, false, 10, 0)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %+v %v", list, err)
	}
	chat := list[0]
	if !chat.IsSaved || chat.PeerID != nil || chat.ConversationType != models.ConversationDirect ||
		chat.UnreadCount != 0 || chat.LastMessageRead || chat.MembersCount != 1 {
		t.Fatalf("saved chat: %+v", chat)
	}
	// обычный личный чат «Избранным» не считается; в «Избранном» нет собеседников для presence
	other := uuid.New()
	if _, err = e.msgs.Send(ctx, me, service.SendMessageInput{RecipientID: &other, Content: "hi"}); err != nil {
		t.Fatal(err)
	}
	list, _ = e.convs.List(ctx, me, false, 10, 0)
	saved := 0
	for _, c := range list {
		if c.IsSaved {
			saved++
		}
	}
	if saved != 1 {
		t.Fatalf("saved chats = %d", saved)
	}
	peers, _ := e.convs.DirectPeers(ctx, me)
	if len(peers) != 1 || peers[0] != other {
		t.Fatalf("direct peers = %v", peers)
	}

	// закреп в «Избранном» работает, но строки «закрепил» нет
	if _, err = e.msgs.PinMessage(ctx, me, id, first.ID, models.PinScopeAll); err != nil {
		t.Fatal(err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM messages WHERE conversation_id = $1 AND kind = 'system'`, id); n != 0 {
		t.Fatalf("system messages in saved = %d", n)
	}
	if pins, _ := e.msgs.ListPins(ctx, me, id); len(pins) != 1 {
		t.Fatalf("pins = %d", len(pins))
	}

	// прочтение своих сообщений не создаёт уведомлений авторам
	if err = e.msgs.MarkRead(ctx, me, id, second.ID); err != nil {
		t.Fatal(err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->>'event_type' = 'message.read'
		AND jsonb_array_length(payload->'payload'->'author_ids') > 0 AND event_key = $1`, id.String()); n != 0 {
		t.Fatalf("message.read with authors = %d", n)
	}
}
