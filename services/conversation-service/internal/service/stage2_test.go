package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
)

func TestListPageAroundAndAfter(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()

	var convID uuid.UUID
	sent := make([]*models.Message, 0, 10)
	for i := 0; i < 10; i++ {
		in := service.SendMessageInput{Content: fmt.Sprintf("m%d", i)}
		if i == 0 {
			in.RecipientID = &b
		} else {
			in.ConversationID = &convID
		}
		m, err := e.msgs.Send(ctx, a, in)
		if err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
		convID = m.ConversationID
		sent = append(sent, m)
	}

	// взаимоисключающие курсоры
	if _, err := e.msgs.ListPage(ctx, b, convID, service.ListMessagesQuery{
		Before: &sent[5].ID, After: &sent[3].ID, Limit: 5,
	}); !errors.Is(err, apperror.ErrIncorrectData) {
		t.Fatalf("mutually exclusive cursors: %v", err)
	}

	// around в середине (sent[4], limit=5): 2 новее (6,5), цель (4), 2 старее (3,2)
	aroundMid, err := e.msgs.ListPage(ctx, b, convID, service.ListMessagesQuery{Around: &sent[4].ID, Limit: 5})
	if err != nil {
		t.Fatalf("around mid: %v", err)
	}
	if !aroundMid.HasOlder || !aroundMid.HasNewer || len(aroundMid.Items) != 5 {
		t.Fatalf("around mid meta: len=%d older=%v newer=%v", len(aroundMid.Items), aroundMid.HasOlder, aroundMid.HasNewer)
	}
	wantMid := []string{"m6", "m5", "m4", "m3", "m2"}
	for i, w := range wantMid {
		if *aroundMid.Items[i].Content != w {
			t.Fatalf("aroundMid[%d] = %q, want %q", i, *aroundMid.Items[i].Content, w)
		}
	}

	// after от верхнего края окна (m6) возвращает 9,8,7 в порядке DESC
	afterPage, err := e.msgs.ListPage(ctx, b, convID, service.ListMessagesQuery{After: &sent[6].ID, Limit: 5})
	if err != nil {
		t.Fatalf("after: %v", err)
	}
	if afterPage.HasNewer || !afterPage.HasOlder || len(afterPage.Items) != 3 {
		t.Fatalf("after meta: len=%d older=%v newer=%v", len(afterPage.Items), afterPage.HasOlder, afterPage.HasNewer)
	}
	wantAfter := []string{"m9", "m8", "m7"}
	for i, w := range wantAfter {
		if *afterPage.Items[i].Content != w {
			t.Fatalf("after[%d] = %q, want %q", i, *afterPage.Items[i].Content, w)
		}
	}

	// after с лимитом меньше остатка: hasNewer=true и ближайшие к курсору
	afterShort, err := e.msgs.ListPage(ctx, b, convID, service.ListMessagesQuery{After: &sent[6].ID, Limit: 2})
	if err != nil || !afterShort.HasNewer || !afterShort.HasOlder || len(afterShort.Items) != 2 {
		t.Fatalf("after short: %+v %v", afterShort, err)
	}
	if *afterShort.Items[0].Content != "m8" || *afterShort.Items[1].Content != "m7" {
		t.Fatalf("after short items: %q, %q", *afterShort.Items[0].Content, *afterShort.Items[1].Content)
	}

	// around у самого свежего (sent[9], limit=5): бюджет сдвигается в сторону старых (9,8,7,6,5)
	aroundTail, err := e.msgs.ListPage(ctx, b, convID, service.ListMessagesQuery{Around: &sent[9].ID, Limit: 5})
	if err != nil || aroundTail.HasNewer || !aroundTail.HasOlder || len(aroundTail.Items) != 5 {
		t.Fatalf("around tail: %+v %v", aroundTail, err)
	}
	if *aroundTail.Items[0].Content != "m9" || *aroundTail.Items[4].Content != "m5" {
		t.Fatalf("around tail range: %q..%q", *aroundTail.Items[0].Content, *aroundTail.Items[4].Content)
	}

	// неизвестный around -> ErrNotFound
	unknown := uuid.New()
	if _, err := e.msgs.ListPage(ctx, b, convID, service.ListMessagesQuery{Around: &unknown, Limit: 5}); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("around unknown: %v", err)
	}
}

func TestClearHistoryKeepsChatAndSettings(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	me, peer := uuid.New(), uuid.New()

	m1, err := e.msgs.Send(ctx, peer, service.SendMessageInput{RecipientID: &me, Content: "до очистки"})
	if err != nil {
		t.Fatal(err)
	}
	convID := m1.ConversationID
	if err := e.msgs.MarkRead(ctx, me, convID, m1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.convs.SetPinned(ctx, me, convID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := e.msgs.PinMessage(ctx, me, convID, m1.ID, models.PinScopeMe); err != nil {
		t.Fatal(err)
	}

	// посторонний не может очистить
	if err := e.convs.ClearHistory(ctx, uuid.New(), convID); !errors.Is(err, apperror.ErrNotMember) {
		t.Fatalf("outsider clear: %v", err)
	}
	if err := e.convs.ClearHistory(ctx, me, convID); err != nil {
		t.Fatalf("clear history: %v", err)
	}

	// чат остаётся в списке с сохранённым закрепом, но без last_message и непрочитанных
	list, err := e.convs.List(ctx, me, false, 10, 0)
	if err != nil || len(list) != 1 {
		t.Fatalf("list after clear: %+v %v", list, err)
	}
	if list[0].LastMessage != nil || list[0].UnreadCount != 0 || list[0].PinnedAt == nil {
		t.Fatalf("cleared summary: %+v", list[0])
	}
	// история и личные закрепы пусты
	if msgs, err := e.msgs.List(ctx, me, convID, nil, 50); err != nil || len(msgs) != 0 {
		t.Fatalf("messages after clear: %d %v", len(msgs), err)
	}
	if pins, err := e.msgs.ListPins(ctx, me, convID); err != nil || len(pins) != 0 {
		t.Fatalf("pins after clear: %d %v", len(pins), err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM pinned_messages WHERE conversation_id = $1 AND owner_id = $2`, convID, me); n != 0 {
		t.Fatalf("personal pins left in db = %d", n)
	}

	// у собеседника история и статус прочтения не затронуты
	peerList, err := e.convs.List(ctx, peer, false, 10, 0)
	if err != nil || len(peerList) != 1 || peerList[0].LastMessage == nil || !peerList[0].LastMessageRead {
		t.Fatalf("peer summary after clear: %+v %v", peerList, err)
	}

	// событие conversation.history.cleared ушло только очистившему
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->>'event_type' = 'conversation.history.cleared'
		AND payload->'payload'->'member_ids' = to_jsonb(ARRAY[$1::text])`, me.String()); n != 1 {
		t.Fatalf("history cleared events = %d", n)
	}

	// новые сообщения после очистки видны и считаются непрочитанными
	m2, err := e.msgs.Send(ctx, peer, service.SendMessageInput{ConversationID: &convID, Content: "после очистки"})
	if err != nil {
		t.Fatal(err)
	}
	afterList, err := e.convs.List(ctx, me, false, 10, 0)
	if err != nil || len(afterList) != 1 || afterList[0].LastMessage == nil || afterList[0].LastMessage.ID != m2.ID || afterList[0].UnreadCount != 1 {
		t.Fatalf("summary after new msg: %+v %v", afterList, err)
	}
}

func TestSilentMessageAndMemberIDs(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a, b, outsider := uuid.New(), uuid.New(), uuid.New()

	m, err := e.msgs.Send(ctx, a, service.SendMessageInput{RecipientID: &b, Content: "тихо", IsSilent: true})
	if err != nil || !m.IsSilent {
		t.Fatalf("send silent: %+v %v", m, err)
	}
	got, err := e.msgs.Get(ctx, b, m.ID)
	if err != nil || !got.IsSilent {
		t.Fatalf("get silent: %+v %v", got, err)
	}
	list, err := e.convs.List(ctx, b, false, 10, 0)
	if err != nil || len(list) != 1 || list[0].LastMessage == nil || !list[0].LastMessage.IsSilent {
		t.Fatalf("summary last message silent: %+v %v", list, err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->>'event_type' = 'message.created'
		AND payload->'payload'->>'is_silent' = 'true'`); n != 1 {
		t.Fatalf("silent outbox events = %d", n)
	}

	// MemberIDs для typing
	ids, err := e.convs.MemberIDs(ctx, a, m.ConversationID)
	if err != nil || len(ids) != 2 {
		t.Fatalf("member ids: %v %v", ids, err)
	}
	if _, err := e.convs.MemberIDs(ctx, outsider, m.ConversationID); !errors.Is(err, apperror.ErrNotMember) {
		t.Fatalf("outsider member ids: %v", err)
	}
}

func TestMessageReactionsLifecycleAndLimits(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a, b, outsider := uuid.New(), uuid.New(), uuid.New()

	m, err := e.msgs.Send(ctx, a, service.SendMessageInput{RecipientID: &b, Content: "привет"})
	if err != nil {
		t.Fatal(err)
	}

	// невалидные эмодзи
	for _, bad := range []string{"", "   ", "ok", "👍 🔥", "123"} {
		if _, err := e.msgs.AddReaction(ctx, a, m.ID, bad); !errors.Is(err, apperror.ErrIncorrectData) {
			t.Fatalf("bad emoji %q: %v", bad, err)
		}
	}
	if _, err := e.msgs.AddReaction(ctx, outsider, m.ID, "👍"); !errors.Is(err, apperror.ErrNotMember) {
		t.Fatalf("outsider reaction: %v", err)
	}

	// добавление + идемпотентность
	r1, err := e.msgs.AddReaction(ctx, a, m.ID, "👍")
	if err != nil || len(r1) != 1 || r1[0].Count != 1 || !r1[0].Me {
		t.Fatalf("add a: %+v %v", r1, err)
	}
	// повтор не порождает второе событие
	rDup, err := e.msgs.AddReaction(ctx, a, m.ID, "👍")
	if err != nil || len(rDup) != 1 || rDup[0].Count != 1 {
		t.Fatalf("dup a: %+v %v", rDup, err)
	}
	r2, err := e.msgs.AddReaction(ctx, b, m.ID, "👍")
	if err != nil || len(r2) != 1 || r2[0].Count != 2 || !r2[0].Me || len(r2[0].UserIDs) != 2 {
		t.Fatalf("add b: %+v %v", r2, err)
	}

	// реакции прикрепляются в ListPage, Get и Edit
	page, err := e.msgs.ListPage(ctx, a, m.ConversationID, service.ListMessagesQuery{Limit: 10})
	if err != nil || len(page.Items) != 1 || len(page.Items[0].Reactions) != 1 || page.Items[0].Reactions[0].Count != 2 || !page.Items[0].Reactions[0].Me {
		t.Fatalf("list reactions: %+v %v", page, err)
	}
	edited, err := e.msgs.Edit(ctx, a, m.ID, "привет всем", nil)
	if err != nil || len(edited.Reactions) != 1 || edited.Reactions[0].Count != 2 {
		t.Fatalf("edit keeps reactions: %+v %v", edited, err)
	}

	// лимит 5 реакций от одного пользователя на сообщение
	for _, em := range []string{"❤️", "😂", "🔥", "1️⃣"} {
		if _, err := e.msgs.AddReaction(ctx, a, m.ID, em); err != nil {
			t.Fatalf("add %s: %v", em, err)
		}
	}
	if _, err := e.msgs.AddReaction(ctx, a, m.ID, "😮"); !errors.Is(err, apperror.ErrReactionLimit) {
		t.Fatalf("6th reaction by same user: %v", err)
	}

	// снятие реакции
	afterRemove, err := e.msgs.RemoveReaction(ctx, a, m.ID, "👍")
	if err != nil || len(afterRemove) != 5 || afterRemove[0].Count != 1 || afterRemove[0].Me {
		t.Fatalf("remove reaction: %+v %v", afterRemove, err)
	}

	// события: 2 ("👍" от a и b) + 4 (доп. эмодзи a) + 1 (снятие "👍") = 7
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->>'event_type' = 'message.reaction.updated'`); n != 7 {
		t.Fatalf("reaction events = %d, want 7", n)
	}

	// удаление сообщения чистит реакции и запрещает новые
	if err := e.msgs.Delete(ctx, a, m.ID); err != nil {
		t.Fatal(err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM message_reactions WHERE message_id = $1`, m.ID); n != 0 {
		t.Fatalf("reactions left after delete = %d", n)
	}
	if _, err := e.msgs.AddReaction(ctx, b, m.ID, "👍"); !errors.Is(err, apperror.ErrMessageDeleted) {
		t.Fatalf("reaction on deleted: %v", err)
	}
}
