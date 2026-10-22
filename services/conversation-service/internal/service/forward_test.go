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

// forwardEnv: у me два чата - с автором исходных сообщений (src) и с другом (dst)
func forwardEnv(t *testing.T, e *env) (me, author, friend, src, dst uuid.UUID, msgs []*models.Message) {
	t.Helper()
	ctx := context.Background()
	me, author, friend = uuid.New(), uuid.New(), uuid.New()
	first, err := e.msgs.Send(ctx, author, service.SendMessageInput{RecipientID: &me, Content: "первое"})
	if err != nil {
		t.Fatal(err)
	}
	src = first.ConversationID
	second, _ := e.msgs.Send(ctx, author, service.SendMessageInput{ConversationID: &src, Content: "второе"})
	hi, _ := e.msgs.Send(ctx, me, service.SendMessageInput{RecipientID: &friend, Content: "привет"})
	return me, author, friend, src, hi.ConversationID, []*models.Message{first, second}
}

func history(t *testing.T, e *env, user, convID uuid.UUID) []models.Message {
	t.Helper()
	list, err := e.msgs.List(context.Background(), user, convID, nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	return list
}

func TestForwardCopies(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	me, author, friend, _, dst, msgs := forwardEnv(t, e)

	// порядок в запросе не важен: копии идут в порядке оригиналов
	res, err := e.msgs.Forward(ctx, me, service.ForwardInput{
		MessageIDs: []uuid.UUID{msgs[1].ID, msgs[0].ID}, ConversationIDs: []uuid.UUID{dst}, ToSaved: true,
	})
	if err != nil || len(res) != 2 {
		t.Fatalf("forward: %+v %v", res, err)
	}
	got := history(t, e, friend, dst)
	if len(got) != 3 || *got[1].Content != "первое" || *got[0].Content != "второе" {
		t.Fatalf("friend history: %+v", got)
	}
	fwd := got[0]
	if fwd.SenderID != me || !fwd.IsForwarded || fwd.ForwardedFromUserID == nil || *fwd.ForwardedFromUserID != author ||
		fwd.ReplyToMessageID != nil {
		t.Fatalf("copy: %+v", fwd)
	}
	// «Избранное» создалось и получило копии
	list, _ := e.convs.List(ctx, me, false, 10, 0)
	var saved *models.ConversationSummary
	for i := range list {
		if list[i].IsSaved {
			saved = &list[i]
		}
	}
	if saved == nil || res[1].ConversationID != saved.ID || len(history(t, e, me, saved.ID)) != 2 {
		t.Fatalf("saved: %+v %+v", saved, res)
	}

	// копия независима: шифртекст свой, правка и удаление оригинала её не меняют
	if n := count(t, e.pool, `SELECT COUNT(*) FROM messages a JOIN messages b ON a.content_enc = b.content_enc
		WHERE a.id = $1 AND b.id <> a.id`, msgs[1].ID); n != 0 {
		t.Fatal("copy shares ciphertext with the original")
	}
	if _, err = e.msgs.Edit(ctx, author, msgs[1].ID, "исправлено"); err != nil {
		t.Fatal(err)
	}
	if err = e.msgs.Delete(ctx, author, msgs[0].ID); err != nil {
		t.Fatal(err)
	}
	got = history(t, e, friend, dst)
	if *got[0].Content != "второе" || got[1].IsDeleted {
		t.Fatalf("copies changed with originals: %+v", got[:2])
	}

	// пересланное нельзя править; удалить своё - можно
	if _, err = e.msgs.Edit(ctx, me, fwd.ID, "моё"); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("edit forwarded: %v", err)
	}

	// событие несёт автора и не несёт текста в открытом виде
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->>'event_type' = 'message.created'
		AND payload->'payload'->>'is_forwarded' = 'true' AND payload->'payload'->>'forwarded_from_user_id' = $1
		AND payload->'payload'->>'content' IS NULL`, author.String()); n != 4 {
		t.Fatalf("forward events = %d", n)
	}
}

func TestForwardAuthor(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	me, author, friend, _, dst, msgs := forwardEnv(t, e)
	one := []uuid.UUID{msgs[0].ID}

	// «без указания автора» - обычное сообщение
	if _, err := e.msgs.Forward(ctx, me, service.ForwardInput{MessageIDs: one, ConversationIDs: []uuid.UUID{dst}, HideAuthor: true}); err != nil {
		t.Fatal(err)
	}
	if c := history(t, e, friend, dst)[0]; c.IsForwarded || c.ForwardedFromUserID != nil {
		t.Fatalf("hide author: %+v", c)
	}

	// автор скрывает имя - «скрытый автор» вопреки выбору пересылающего
	e.users.hideForward[author] = true
	if _, err := e.msgs.Forward(ctx, me, service.ForwardInput{MessageIDs: one, ConversationIDs: []uuid.UUID{dst}}); err != nil {
		t.Fatal(err)
	}
	hidden := history(t, e, friend, dst)[0]
	if !hidden.IsForwarded || hidden.ForwardedFromUserID != nil {
		t.Fatalf("hidden author: %+v", hidden)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->'payload'->>'message_id' = $1
		AND payload::text LIKE '%' || $2 || '%'`, hidden.ID.String(), author.String()); n != 0 {
		t.Fatal("hidden author leaked into the event")
	}

	// цепочка: друг пересылает дальше - автор остаётся первым (или скрытым)
	e.users.hideForward[author] = false
	if _, err := e.msgs.Forward(ctx, me, service.ForwardInput{MessageIDs: one, ConversationIDs: []uuid.UUID{dst}}); err != nil {
		t.Fatal(err)
	}
	shown := history(t, e, friend, dst)[0]
	third := uuid.New()
	hi, _ := e.msgs.Send(ctx, friend, service.SendMessageInput{RecipientID: &third, Content: "x"})
	if _, err := e.msgs.Forward(ctx, friend, service.ForwardInput{
		MessageIDs: []uuid.UUID{shown.ID, hidden.ID}, ConversationIDs: []uuid.UUID{hi.ConversationID},
	}); err != nil {
		t.Fatal(err)
	}
	chain := history(t, e, third, hi.ConversationID)
	if chain[0].ForwardedFromUserID == nil || *chain[0].ForwardedFromUserID != author || chain[1].ForwardedFromUserID != nil {
		t.Fatalf("chain: %+v", chain[:2])
	}

	// user-service недоступен - пересылка отклоняется, имя не раскрывается
	e.users.down = true
	if _, err := e.msgs.Forward(ctx, me, service.ForwardInput{MessageIDs: one, ConversationIDs: []uuid.UUID{dst}}); !errors.Is(err, apperror.ErrUserServiceError) {
		t.Fatalf("user-service down: %v", err)
	}
}

func TestForwardRules(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	me, _, friend, src, dst, msgs := forwardEnv(t, e)
	one := []uuid.UUID{msgs[0].ID}
	to := []uuid.UUID{dst}

	cases := map[string]struct {
		in   service.ForwardInput
		want error
	}{
		"no messages":    {service.ForwardInput{ConversationIDs: to}, apperror.ErrIncorrectData},
		"no targets":     {service.ForwardInput{MessageIDs: one}, apperror.ErrIncorrectData},
		"too many chats": {service.ForwardInput{MessageIDs: one, ConversationIDs: []uuid.UUID{dst, uuid.New(), uuid.New(), uuid.New(), uuid.New()}, ToSaved: true}, apperror.ErrIncorrectData},
		"not my chat":    {service.ForwardInput{MessageIDs: one, ConversationIDs: []uuid.UUID{uuid.New()}}, apperror.ErrNotFound},
		"invisible msg":  {service.ForwardInput{MessageIDs: []uuid.UUID{uuid.New()}, ConversationIDs: to}, apperror.ErrNotFound},
	}
	many := make([]uuid.UUID, 21)
	for i := range many {
		many[i] = uuid.New()
	}
	cases["too many messages"] = struct {
		in   service.ForwardInput
		want error
	}{service.ForwardInput{MessageIDs: many, ConversationIDs: to}, apperror.ErrIncorrectData}
	for name, c := range cases {
		if _, err := e.msgs.Forward(ctx, me, c.in); !errors.Is(err, c.want) {
			t.Errorf("%s: %v", name, err)
		}
	}

	// сообщения из разных чатов
	mine := history(t, e, me, dst)[0]
	if _, err := e.msgs.Forward(ctx, me, service.ForwardInput{MessageIDs: []uuid.UUID{msgs[0].ID, mine.ID}, ConversationIDs: to}); !errors.Is(err, apperror.ErrIncorrectData) {
		t.Fatalf("mixed chats: %v", err)
	}
	// удалённое
	if err := e.msgs.Delete(ctx, me, mine.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.msgs.Forward(ctx, me, service.ForwardInput{MessageIDs: []uuid.UUID{mine.ID}, ConversationIDs: []uuid.UUID{src}}); !errors.Is(err, apperror.ErrMessageDeleted) {
		t.Fatalf("deleted: %v", err)
	}
	// блокировка в целевом личном чате
	e.users.blockedBySender[[2]uuid.UUID{friend, me}] = true
	if _, err := e.msgs.Forward(ctx, me, service.ForwardInput{MessageIDs: one, ConversationIDs: to}); !errors.Is(err, apperror.ErrBlockedByThem) {
		t.Fatalf("blocked: %v", err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM messages WHERE is_forwarded`); n != 0 {
		t.Fatalf("refused forwards wrote %d messages", n)
	}
}
