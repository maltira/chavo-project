package service_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/testutil"
	"github.com/maltira/chavo-project-backend/services/conversation-service/pkg/crypto"
)

type fakeUsers struct {
	blockedBySender map[[2]uuid.UUID]bool // {sender, recipient}: sender заблокировал recipient
	missing         map[uuid.UUID]bool    // пользователя нет в user-service
	noInvites       map[uuid.UUID]bool    // пользователь запретил приглашения в группы
}

func (f *fakeUsers) CheckMessagingAllowed(_ context.Context, s, r uuid.UUID) error {
	if f.blockedBySender[[2]uuid.UUID{s, r}] {
		return apperror.ErrBlockedByMe
	}
	if f.blockedBySender[[2]uuid.UUID{r, s}] {
		return apperror.ErrBlockedByThem
	}
	return nil
}
func (f *fakeUsers) UserExists(_ context.Context, id uuid.UUID) (bool, error) {
	return !f.missing[id], nil
}

func (f *fakeUsers) GroupInviteAllowed(_ context.Context, id uuid.UUID) (bool, error) {
	return !f.noInvites[id], nil
}

type env struct {
	pool   *pgxpool.Pool
	msgs   service.MessageService
	convs  service.ConversationService
	groups service.GroupService
	joins  service.JoinService
	users  *fakeUsers
}

func setup(t *testing.T) *env {
	t.Helper()
	pool := testutil.Pool(t, "service_test")

	cipher, _ := crypto.NewCipher(bytes.Repeat([]byte{7}, 32))
	db := repository.NewDB(pool)
	cr, mr, or := repository.NewConversationRepository(), repository.NewMessageRepository(), repository.NewOutboxRepository()
	users := &fakeUsers{
		blockedBySender: map[[2]uuid.UUID]bool{},
		missing:         map[uuid.UUID]bool{},
		noInvites:       map[uuid.UUID]bool{},
	}
	return &env{
		pool:   pool,
		msgs:   service.NewMessageService(db, cr, mr, or, users, cipher, zap.NewNop()),
		convs:  service.NewConversationService(db, cr, cipher),
		groups: service.NewGroupService(db, cr, or, users),
		joins:  service.NewJoinService(db, cr, repository.NewJoinRequestRepository(), or),
		users:  users,
	}
}

func count(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestConcurrentFirstMessagesCreateSingleDirect(t *testing.T) {
	e := setup(t)
	a, b := uuid.New(), uuid.New()

	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for i := 0; i < 40; i++ {
		from, to := a, b
		if i%2 == 1 {
			from, to = b, a
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.msgs.Send(context.Background(), from, service.SendMessageInput{RecipientID: &to, Content: "hi"})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("send failed: %v", err)
		}
	}

	if n := count(t, e.pool, `SELECT COUNT(*) FROM conversations`); n != 1 {
		t.Fatalf("conversations = %d, want 1", n)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM conversation_members WHERE member_role = 'admin'`); n != 2 {
		t.Fatalf("admin members = %d, want 2", n)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM messages`); n != 40 {
		t.Fatalf("messages = %d, want 40", n)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->>'event_type' = 'conversation.created'`); n != 1 {
		t.Fatalf("conversation.created events = %d, want 1", n)
	}
}

func TestBlockedSendCreatesNothing(t *testing.T) {
	e := setup(t)
	a, b := uuid.New(), uuid.New()
	e.users.blockedBySender[[2]uuid.UUID{a, b}] = true

	_, err := e.msgs.Send(context.Background(), a, service.SendMessageInput{RecipientID: &b, Content: "x"})
	if !errors.Is(err, apperror.ErrBlockedByMe) {
		t.Fatalf("a->b: %v", err)
	}
	_, err = e.msgs.Send(context.Background(), b, service.SendMessageInput{RecipientID: &a, Content: "x"})
	if !errors.Is(err, apperror.ErrBlockedByThem) {
		t.Fatalf("b->a: %v", err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM conversations`); n != 0 {
		t.Fatalf("conversations = %d, want 0", n)
	}
}

func TestSendValidation(t *testing.T) {
	e := setup(t)
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	ctx := context.Background()

	for name, in := range map[string]service.SendMessageInput{
		"both ids": {ConversationID: &c, RecipientID: &b, Content: "x"},
		"no ids":   {Content: "x"},
		"blank":    {RecipientID: &b, Content: "   "},
		"too long": {RecipientID: &b, Content: strings.Repeat("я", 4097)},
		"to self":  {RecipientID: &a, Content: "x"},
	} {
		if _, err := e.msgs.Send(ctx, a, in); !errors.Is(err, apperror.ErrIncorrectData) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := e.msgs.Send(ctx, a, service.SendMessageInput{ConversationID: &c, Content: "x"}); !errors.Is(err, apperror.ErrNotFound) {
		t.Errorf("unknown conversation: %v", err)
	}
}

func TestMessageLifecycle(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a, b, outsider := uuid.New(), uuid.New(), uuid.New()

	m1, err := e.msgs.Send(ctx, a, service.SendMessageInput{RecipientID: &b, Content: "секретный текст"})
	if err != nil {
		t.Fatal(err)
	}
	convID := m1.ConversationID

	// Plaintext нигде не хранится: ни в messages, ни в outbox (там только шифртекст для publisher'а).
	if n := count(t, e.pool, `SELECT COUNT(*) FROM messages WHERE position('секретный'::bytea in content_enc) > 0`); n != 0 {
		t.Fatal("plaintext found in content_enc")
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload::text LIKE '%секретный%'`); n != 0 {
		t.Fatal("plaintext found in outbox")
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->>'event_type' = 'message.created' AND payload->'payload'->>'content_enc' IS NOT NULL`); n != 1 {
		t.Fatalf("message.created events with content_enc = %d, want 1", n)
	}

	// reply: в тот же чат можно, в чужой — нет.
	m2, err := e.msgs.Send(ctx, b, service.SendMessageInput{ConversationID: &convID, Content: "ответ", ReplyToMessageID: &m1.ID})
	if err != nil {
		t.Fatal(err)
	}
	c := uuid.New()
	other, err := e.msgs.Send(ctx, a, service.SendMessageInput{RecipientID: &c, Content: "другой чат"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.msgs.Send(ctx, a, service.SendMessageInput{ConversationID: &convID, Content: "x", ReplyToMessageID: &other.ID}); !errors.Is(err, apperror.ErrInvalidReply) {
		t.Fatalf("cross-conversation reply: %v", err)
	}

	// Посторонний не читает и не пишет.
	if _, err = e.msgs.List(ctx, outsider, convID, nil, 10); !errors.Is(err, apperror.ErrNotMember) {
		t.Fatalf("outsider list: %v", err)
	}
	if _, err = e.msgs.Send(ctx, outsider, service.SendMessageInput{ConversationID: &convID, Content: "x"}); !errors.Is(err, apperror.ErrNotMember) {
		t.Fatalf("outsider send: %v", err)
	}

	// Edit: только автор.
	if _, err = e.msgs.Edit(ctx, b, m1.ID, "чужое"); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("edit by non-author: %v", err)
	}
	edited, err := e.msgs.Edit(ctx, a, m1.ID, "исправлено")
	if err != nil || !edited.IsEdited || *edited.Content != "исправлено" {
		t.Fatalf("edit: %+v %v", edited, err)
	}

	// Delete в direct: чужое нельзя, своё можно, повтор идемпотентен, edit после удаления запрещён.
	if err = e.msgs.Delete(ctx, a, m2.ID); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("delete foreign in direct: %v", err)
	}
	if err = e.msgs.Delete(ctx, b, m2.ID); err != nil {
		t.Fatal(err)
	}
	if err = e.msgs.Delete(ctx, b, m2.ID); err != nil {
		t.Fatalf("repeat delete: %v", err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->>'event_type' = 'message.deleted'`); n != 1 {
		t.Fatalf("message.deleted events = %d, want 1", n)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->'payload'->>'message_id' = $1 AND payload->'payload'->>'content_enc' IS NOT NULL`, m2.ID.String()); n != 0 {
		t.Fatalf("deleted message keeps content_enc in %d outbox events", n)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->'payload'->>'message_id' = $1 AND payload->'payload'->>'content_enc' IS NOT NULL`, m1.ID.String()); n == 0 {
		t.Fatal("scrub must not touch other messages")
	}
	if _, err = e.msgs.Edit(ctx, b, m2.ID, "x"); !errors.Is(err, apperror.ErrMessageDeleted) {
		t.Fatalf("edit deleted: %v", err)
	}

	list, err := e.msgs.List(ctx, a, convID, nil, 10)
	if err != nil || len(list) != 2 {
		t.Fatalf("list: %d %v", len(list), err)
	}
	if list[0].ID != m2.ID || !list[0].IsDeleted || list[0].Content != nil {
		t.Fatalf("deleted message must come newest-first without content: %+v", list[0])
	}
}

func TestGroupAdminCanDeleteForeignMessage(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, member := uuid.New(), uuid.New()

	var convID uuid.UUID
	if err := e.pool.QueryRow(ctx, `INSERT INTO conversations (conversation_type, name) VALUES ('group','g') RETURNING id`).Scan(&convID); err != nil {
		t.Fatal(err)
	}
	for u, role := range map[uuid.UUID]string{admin: "admin", member: "member"} {
		if _, err := e.pool.Exec(ctx, `INSERT INTO conversation_members (conversation_id, user_id, member_role) VALUES ($1,$2,$3)`, convID, u, role); err != nil {
			t.Fatal(err)
		}
	}

	msg, err := e.msgs.Send(ctx, member, service.SendMessageInput{ConversationID: &convID, Content: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if err = e.msgs.Delete(ctx, admin, msg.ID); err != nil {
		t.Fatalf("admin delete: %v", err)
	}
	adminMsg, _ := e.msgs.Send(ctx, admin, service.SendMessageInput{ConversationID: &convID, Content: "от админа"})
	if err = e.msgs.Delete(ctx, member, adminMsg.ID); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("member deleting admin message: %v", err)
	}
}

func TestPaginationAndReadReceipts(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()

	var ids []uuid.UUID
	for i := 0; i < 5; i++ {
		m, err := e.msgs.Send(ctx, a, service.SendMessageInput{RecipientID: &b, Content: strings.Repeat("m", i+1)})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, m.ID)
	}
	conv := mustConv(t, e, a)

	page1, _ := e.msgs.List(ctx, a, conv, nil, 2)
	page2, _ := e.msgs.List(ctx, a, conv, &page1[1].ID, 2)
	page3, _ := e.msgs.List(ctx, a, conv, &page2[1].ID, 2)
	got := []uuid.UUID{page1[0].ID, page1[1].ID, page2[0].ID, page2[1].ID, page3[0].ID}
	if len(page3) != 1 {
		t.Fatalf("page3 = %d", len(page3))
	}
	for i, id := range got {
		if id != ids[4-i] {
			t.Fatalf("pos %d: got %s want %s", i, id, ids[4-i])
		}
	}

	// Непрочитанные у b: 5. После read до ids[2] — 2.
	summary, err := e.convs.Get(ctx, b, conv)
	if err != nil || summary.UnreadCount != 5 || summary.PeerID == nil || *summary.PeerID != a {
		t.Fatalf("summary: %+v %v", summary, err)
	}
	if err = e.msgs.MarkRead(ctx, b, conv, ids[2]); err != nil {
		t.Fatal(err)
	}
	summary, _ = e.convs.Get(ctx, b, conv)
	if summary.UnreadCount != 2 || summary.LastMessage == nil || summary.LastMessage.ID != ids[4] {
		t.Fatalf("after read: %+v", summary)
	}

	// Курсор не откатывается назад и не порождает лишнее событие.
	if err = e.msgs.MarkRead(ctx, b, conv, ids[0]); err != nil {
		t.Fatal(err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->>'event_type' = 'message.read'`); n != 1 {
		t.Fatalf("message.read events = %d, want 1", n)
	}

	readers, _ := e.msgs.Readers(ctx, a, conv, ids[1])
	if len(readers) != 1 || readers[0] != b {
		t.Fatalf("readers of ids[1] = %v", readers)
	}
	readers, _ = e.msgs.Readers(ctx, a, conv, ids[3])
	if len(readers) != 0 {
		t.Fatalf("readers of ids[3] = %v", readers)
	}

	list, _ := e.convs.List(ctx, a, 10, 0)
	if len(list) != 1 || list[0].UnreadCount != 0 {
		t.Fatalf("list for a: %+v", list)
	}
}

func mustConv(t *testing.T, e *env, user uuid.UUID) uuid.UUID {
	t.Helper()
	list, err := e.convs.List(context.Background(), user, 1, 0)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v %v", list, err)
	}
	return list[0].ID
}
