package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
)

func TestDeleteDirectForMe(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	me, peer := uuid.New(), uuid.New()

	first, _ := e.msgs.Send(ctx, peer, service.SendMessageInput{RecipientID: &me, Content: "старое"})
	id := first.ConversationID
	if _, err := e.msgs.Send(ctx, me, service.SendMessageInput{ConversationID: &id, Content: "ответ", ReplyToMessageID: &first.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.convs.SetPinned(ctx, me, id, true); err != nil {
		t.Fatal(err)
	}

	if err := e.groups.Delete(ctx, uuid.New(), id, false); !errors.Is(err, apperror.ErrNotMember) {
		t.Fatalf("outsider: %v", err)
	}
	if err := e.groups.Delete(ctx, me, id, false); err != nil {
		t.Fatal(err)
	}

	// у меня чат пропал из списка, история и цитата скрыты, закреп снят; карточка открывается пустой
	if ids := listIDs(t, e, me, false); len(ids) != 0 {
		t.Fatalf("list = %v", ids)
	}
	if msgs, _ := e.msgs.List(ctx, me, id, nil, 50); len(msgs) != 0 {
		t.Fatalf("history = %d", len(msgs))
	}
	if _, err := e.msgs.Get(ctx, me, first.ID); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("old message: %v", err)
	}
	card, err := e.convs.Get(ctx, me, id)
	if err != nil || card.LastMessage != nil || card.UnreadCount != 0 || card.PinnedAt != nil {
		t.Fatalf("card: %+v %v", card, err)
	}

	// у собеседника всё на месте
	if msgs, _ := e.msgs.List(ctx, peer, id, nil, 50); len(msgs) != 2 {
		t.Fatalf("peer history = %d", len(msgs))
	}
	if ids := listIDs(t, e, peer, false); len(ids) != 1 {
		t.Fatalf("peer list = %v", ids)
	}

	// событие - только мне, чат не удалён
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->>'event_type' = 'conversation.deleted'
		AND payload->'payload'->>'for_everyone' = 'false'
		AND payload->'payload'->'member_ids' = to_jsonb(ARRAY[$1::text])`, me.String()); n != 1 {
		t.Fatalf("deleted-for-me events = %d", n)
	}

	// новое сообщение возвращает чат, но только с ним; непрочитанное - одно
	fresh, err := e.msgs.Send(ctx, peer, service.SendMessageInput{ConversationID: &id, Content: "снова привет"})
	if err != nil {
		t.Fatal(err)
	}
	list, _ := e.convs.List(ctx, me, false, 10, 0)
	if len(list) != 1 || list[0].LastMessage == nil || list[0].LastMessage.ID != fresh.ID || list[0].UnreadCount != 1 {
		t.Fatalf("returned chat: %+v", list)
	}
	if msgs, _ := e.msgs.List(ctx, me, id, nil, 50); len(msgs) != 1 || msgs[0].ID != fresh.ID {
		t.Fatalf("history after return = %+v", msgs)
	}

	// писать в очищенный чат можно и самому - через профиль собеседника
	if err := e.groups.Delete(ctx, me, id, false); err != nil {
		t.Fatal(err)
	}
	mine, err := e.msgs.Send(ctx, me, service.SendMessageInput{RecipientID: &peer, Content: "я вернулся"})
	if err != nil || mine.ConversationID != id {
		t.Fatalf("send to cleared: %+v %v", mine, err)
	}
	if msgs, _ := e.msgs.List(ctx, me, id, nil, 50); len(msgs) != 1 {
		t.Fatalf("history after my message = %d", len(msgs))
	}
}

func TestDeleteDirectForEveryone(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	me, peer := uuid.New(), uuid.New()
	m, _ := e.msgs.Send(ctx, peer, service.SendMessageInput{RecipientID: &me, Content: "hi"})

	if err := e.groups.Delete(ctx, me, m.ConversationID, true); err != nil {
		t.Fatal(err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM conversations WHERE id = $1`, m.ConversationID); n != 0 {
		t.Fatal("direct must be deleted")
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM messages WHERE conversation_id = $1`, m.ConversationID); n != 0 {
		t.Fatal("history must be deleted")
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->>'event_type' = 'conversation.deleted'
		AND payload->'payload'->>'for_everyone' = 'true' AND payload->'payload'->>'conversation_type' = 'direct'
		AND jsonb_array_length(payload->'payload'->'member_ids') = 2`); n != 1 {
		t.Fatalf("deleted events = %d", n)
	}

	// следующее сообщение создаёт новый чат
	again, err := e.msgs.Send(ctx, me, service.SendMessageInput{RecipientID: &peer, Content: "заново"})
	if err != nil || again.ConversationID == m.ConversationID {
		t.Fatalf("new direct: %+v %v", again, err)
	}
}
