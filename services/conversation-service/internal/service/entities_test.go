package service_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
)

func ent(typ string, offset, length int) models.Entity {
	return models.Entity{Type: typ, Offset: offset, Length: length}
}

func TestFormattedMessageRoundTrip(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()

	// «привет мир»: жирное «привет», подчёркнутое «мир», курсив поверх всего; вход не по порядку
	in := []models.Entity{ent(models.EntityUnderline, 7, 3), ent(models.EntityItalic, 0, 10), ent(models.EntityBold, 0, 6)}
	want := []models.Entity{ent(models.EntityItalic, 0, 10), ent(models.EntityBold, 0, 6), ent(models.EntityUnderline, 7, 3)}
	m, err := e.msgs.Send(ctx, a, service.SendMessageInput{RecipientID: &b, Content: "привет мир", Entities: in})
	if err != nil || !slices.Equal(m.Entities, want) {
		t.Fatalf("send: %+v %v", m, err)
	}
	id := m.ConversationID

	// история, одно сообщение и последнее сообщение в списке несут разметку
	hist, _ := e.msgs.List(ctx, b, id, nil, 10)
	if len(hist) != 1 || !slices.Equal(hist[0].Entities, want) {
		t.Fatalf("history: %+v", hist)
	}
	one, err := e.msgs.Get(ctx, b, m.ID)
	if err != nil || !slices.Equal(one.Entities, want) {
		t.Fatalf("get: %+v %v", one, err)
	}
	list, _ := e.convs.List(ctx, b, false, 10, 0)
	if !slices.Equal(list[0].LastMessage.Entities, want) {
		t.Fatalf("summary: %+v", list[0].LastMessage)
	}

	// в событии - разметка, а в БД текста по-прежнему нет
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->>'event_type' = 'message.created'
		AND jsonb_array_length(payload->'payload'->'entities') = 3 AND payload->'payload'->>'content' IS NULL`); n != 1 {
		t.Fatalf("created events with entities = %d", n)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM messages WHERE entities::text LIKE '%привет%'`); n != 0 {
		t.Fatal("entities hold text")
	}

	// правка заменяет текст и разметку целиком; без разметки - обычный текст
	edited, err := e.msgs.Edit(ctx, a, m.ID, "новый", []models.Entity{ent(models.EntityBold, 0, 5)})
	if err != nil || !slices.Equal(edited.Entities, []models.Entity{ent(models.EntityBold, 0, 5)}) {
		t.Fatalf("edit: %+v %v", edited, err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->>'event_type' = 'message.updated'
		AND jsonb_array_length(payload->'payload'->'entities') = 1`); n != 1 {
		t.Fatalf("updated events with entities = %d", n)
	}
	plain, err := e.msgs.Edit(ctx, a, m.ID, "новый", nil)
	if err != nil || len(plain.Entities) != 0 {
		t.Fatalf("edit to plain: %+v %v", plain, err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM messages WHERE id = $1 AND entities IS NULL`, m.ID); n != 1 {
		t.Fatal("plain message must store NULL")
	}
}

func TestEntitiesNormalizedAndValidated(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	send := func(content string, entities ...models.Entity) (*models.Message, error) {
		return e.msgs.Send(ctx, a, service.SendMessageInput{RecipientID: &b, Content: content, Entities: entities})
	}

	// пересекающиеся и соседние фрагменты одного вида склеиваются
	m, err := send("0123456789", ent(models.EntityBold, 0, 4), ent(models.EntityBold, 3, 3), ent(models.EntityBold, 6, 2), ent(models.EntityBold, 9, 1))
	if err != nil || !slices.Equal(m.Entities, []models.Entity{ent(models.EntityBold, 0, 8), ent(models.EntityBold, 9, 1)}) {
		t.Fatalf("merge: %+v %v", m, err)
	}
	// смещения в единицах UTF-16: эмодзи занимает две
	if m, err = send("a😀b", ent(models.EntityItalic, 1, 2), ent(models.EntityBold, 3, 1)); err != nil || len(m.Entities) != 2 {
		t.Fatalf("surrogates: %+v %v", m, err)
	}

	bad := map[string][]models.Entity{
		"unknown type":      {ent("strike", 0, 1)},
		"zero length":       {ent(models.EntityBold, 0, 0)},
		"negative length":   {ent(models.EntityBold, 2, -1)},
		"negative offset":   {ent(models.EntityBold, -1, 2)},
		"past the end":      {ent(models.EntityBold, 8, 5)},
		"overflowing range": {ent(models.EntityBold, 1, int(^uint(0)>>1))},
	}
	many := make([]models.Entity, 101)
	for i := range many {
		many[i] = ent(models.EntityBold, i%5*2, 1)
	}
	bad["too many"] = many
	for name, entities := range bad {
		if _, err := send("0123456789", entities...); !errors.Is(err, apperror.ErrIncorrectData) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// граница посреди эмодзи (суррогатной пары) недопустима
	for name, en := range map[string]models.Entity{
		"starts inside": ent(models.EntityBold, 2, 1),
		"ends inside":   ent(models.EntityBold, 0, 2),
	} {
		if _, err := send("a😀b", en); !errors.Is(err, apperror.ErrIncorrectData) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// ошибка в разметке - ничего не сохранено
	if n := count(t, e.pool, `SELECT COUNT(*) FROM messages WHERE content_enc IS NOT NULL`); n != 2 {
		t.Fatalf("messages = %d, want only the 2 valid ones", n)
	}
	// то же правило у правки
	if _, err := e.msgs.Edit(ctx, a, m.ID, "ab", []models.Entity{ent(models.EntityBold, 0, 5)}); !errors.Is(err, apperror.ErrIncorrectData) {
		t.Fatalf("edit past the end: %v", err)
	}
}

func TestForwardKeepsFormatting(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	me, author, friend, _, dst, _ := forwardEnv(t, e)
	src, _ := e.msgs.Send(ctx, author, service.SendMessageInput{RecipientID: &me, Content: "важное слово", Entities: []models.Entity{ent(models.EntityBold, 0, 6)}})
	if _, err := e.msgs.Forward(ctx, me, service.ForwardInput{MessageIDs: []uuid.UUID{src.ID}, ConversationIDs: []uuid.UUID{dst}}); err != nil {
		t.Fatal(err)
	}
	copyMsg := history(t, e, friend, dst)[0]
	if *copyMsg.Content != "важное слово" || !slices.Equal(copyMsg.Entities, []models.Entity{ent(models.EntityBold, 0, 6)}) {
		t.Fatalf("forwarded: %+v", copyMsg)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->'payload'->>'message_id' = $1
		AND jsonb_array_length(payload->'payload'->'entities') = 1`, copyMsg.ID.String()); n != 1 {
		t.Fatalf("forward event entities = %d", n)
	}
}
