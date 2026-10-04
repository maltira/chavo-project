package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
)

func TestInviteInfoAndRegenerate(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, member, guest := uuid.New(), uuid.New(), uuid.New()
	g := createGroup(t, e, admin, "private", member)
	id, token := g.Conversation.ID, *g.InviteToken

	info, err := e.joins.InviteInfo(ctx, guest, token)
	if err != nil || info.ConversationID != id || info.MembersCount != 2 || info.IsMember || info.RequestStatus != nil {
		t.Fatalf("guest info: %+v %v", info, err)
	}
	info, _ = e.joins.InviteInfo(ctx, member, token)
	if !info.IsMember {
		t.Fatal("member must see is_member")
	}
	if _, err = e.joins.InviteInfo(ctx, guest, "wrong-token"); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("wrong token: %v", err)
	}

	// regenerate: только admin приватной группы; старый токен перестаёт работать.
	if _, err = e.joins.RegenerateInvite(ctx, member, id); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("member regenerate: %v", err)
	}
	if _, err = e.joins.RegenerateInvite(ctx, guest, id); !errors.Is(err, apperror.ErrNotMember) {
		t.Fatalf("outsider regenerate: %v", err)
	}
	fresh, err := e.joins.RegenerateInvite(ctx, admin, id)
	if err != nil || fresh == token {
		t.Fatalf("regenerate: %q %v", fresh, err)
	}
	if _, err = e.joins.InviteInfo(ctx, guest, token); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("old token must be dead: %v", err)
	}
	if _, err = e.joins.InviteInfo(ctx, guest, fresh); err != nil {
		t.Fatalf("new token: %v", err)
	}

	pub := createGroup(t, e, admin, "public").Conversation.ID
	if _, err = e.joins.RegenerateInvite(ctx, admin, pub); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("regenerate public: %v", err)
	}
	// Переход в public убивает ссылку.
	pubStr := "public"
	if _, err = e.groups.Update(ctx, admin, id, service.UpdateGroupInput{Visibility: &pubStr}); err != nil {
		t.Fatal(err)
	}
	if _, err = e.joins.InviteInfo(ctx, guest, fresh); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("link of public group: %v", err)
	}
}

func TestRequestJoinRules(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, member, guest := uuid.New(), uuid.New(), uuid.New()
	g := createGroup(t, e, admin, "private", member)
	id, token := g.Conversation.ID, *g.InviteToken
	other := createGroup(t, e, admin, "private")

	if _, err := e.joins.RequestJoin(ctx, guest, id, "wrong"); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("wrong token: %v", err)
	}
	if _, err := e.joins.RequestJoin(ctx, guest, id, *other.InviteToken); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("token of another group: %v", err)
	}
	if _, err := e.joins.RequestJoin(ctx, guest, uuid.New(), token); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("unknown group: %v", err)
	}
	if _, err := e.joins.RequestJoin(ctx, member, id, token); !errors.Is(err, apperror.ErrAlreadyMember) {
		t.Fatalf("member requests: %v", err)
	}
	pub := createGroup(t, e, admin, "public").Conversation.ID
	if _, err := e.joins.RequestJoin(ctx, guest, pub, token); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("request to public group: %v", err)
	}

	jr, err := e.joins.RequestJoin(ctx, guest, id, token)
	if err != nil || jr.Status != "pending" || jr.UserID != guest {
		t.Fatalf("request: %+v %v", jr, err)
	}
	if _, err = e.joins.RequestJoin(ctx, guest, id, token); !errors.Is(err, apperror.ErrJoinRequestExists) {
		t.Fatalf("duplicate: %v", err)
	}
	if info, _ := e.joins.InviteInfo(ctx, guest, token); info.RequestStatus == nil || *info.RequestStatus != "pending" {
		t.Fatalf("info must show pending request: %+v", info)
	}
	if events(t, e, "conversation.join_request.created") != 1 {
		t.Fatalf("created events = %d", events(t, e, "conversation.join_request.created"))
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->>'event_type'='conversation.join_request.created' AND payload->'payload'->'admin_ids' @> to_jsonb($1::text)`, admin.String()); n != 1 {
		t.Fatal("admins must be notified")
	}
}

func TestConcurrentDuplicateRequests(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, guest := uuid.New(), uuid.New()
	g := createGroup(t, e, admin, "private")

	var wg sync.WaitGroup
	results := make(chan error, 15)
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.joins.RequestJoin(ctx, guest, g.Conversation.ID, *g.InviteToken)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	ok := 0
	for err := range results {
		switch {
		case err == nil:
			ok++
		case !errors.Is(err, apperror.ErrJoinRequestExists):
			t.Fatalf("unexpected: %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("successful requests = %d, want 1", ok)
	}
}

func TestApproveRejectFlow(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, member, g1, g2 := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	g := createGroup(t, e, admin, "private", member)
	id, token := g.Conversation.ID, *g.InviteToken

	r1, _ := e.joins.RequestJoin(ctx, g1, id, token)
	r2, _ := e.joins.RequestJoin(ctx, g2, id, token)

	if _, err := e.joins.ListRequests(ctx, member, id, "", 10, 0); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("member lists requests: %v", err)
	}
	if _, err := e.joins.ListRequests(ctx, admin, id, "bogus", 10, 0); !errors.Is(err, apperror.ErrIncorrectData) {
		t.Fatalf("bad status: %v", err)
	}
	list, err := e.joins.ListRequests(ctx, admin, id, "", 10, 0)
	if err != nil || len(list) != 2 {
		t.Fatalf("pending list: %d %v", len(list), err)
	}

	if err = e.joins.Approve(ctx, member, id, r1.ID); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("member approves: %v", err)
	}
	if err = e.joins.Approve(ctx, admin, id, uuid.New()); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("unknown request: %v", err)
	}
	// Заявка из другой группы не одобряется через чужой conversation_id.
	otherGroup := createGroup(t, e, admin, "private").Conversation.ID
	if err = e.joins.Approve(ctx, admin, otherGroup, r1.ID); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("cross-group approve: %v", err)
	}

	if err = e.joins.Approve(ctx, admin, id, r1.ID); err != nil {
		t.Fatal(err)
	}
	if memberRole(t, e, id, g1) != "member" {
		t.Fatal("approved user must become a member")
	}
	if err = e.joins.Approve(ctx, admin, id, r1.ID); !errors.Is(err, apperror.ErrJoinRequestNotPending) {
		t.Fatalf("repeat approve: %v", err)
	}
	if err = e.joins.Reject(ctx, admin, id, r1.ID); !errors.Is(err, apperror.ErrJoinRequestNotPending) {
		t.Fatalf("reject approved: %v", err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->>'event_type'='conversation.join_request.approved' AND payload->'payload'->'member_ids' @> to_jsonb($1::text)`, g1.String()); n != 1 {
		t.Fatal("approved event must notify the new member")
	}

	if err = e.joins.Reject(ctx, admin, id, r2.ID); err != nil {
		t.Fatal(err)
	}
	if memberRole(t, e, id, g2) != "" {
		t.Fatal("rejected user must not become a member")
	}
	if events(t, e, "conversation.join_request.approved") != 1 {
		t.Fatal("reject must not publish approved event")
	}
	// После отказа можно подать новую заявку.
	if _, err = e.joins.RequestJoin(ctx, g2, id, token); err != nil {
		t.Fatalf("re-request after reject: %v", err)
	}
	rejected, _ := e.joins.ListRequests(ctx, admin, id, "rejected", 10, 0)
	if len(rejected) != 1 || rejected[0].UserID != g2 {
		t.Fatalf("rejected list: %+v", rejected)
	}
}

func TestSearchPublicGroups(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, viewer := uuid.New(), uuid.New()
	mk := func(name, visibility string) uuid.UUID {
		res, err := e.groups.Create(ctx, admin, service.CreateGroupInput{Name: name, Visibility: visibility})
		if err != nil {
			t.Fatal(err)
		}
		return res.Conversation.ID
	}
	pubID := mk("Golang Беседка", "public")
	mk("Golang секреты", "private")
	mk("100%_эксперт", "public")
	_ = e.groups.Join(ctx, viewer, pubID)

	res, err := e.convs.SearchPublicGroups(ctx, viewer, "golang", 10, 0)
	if err != nil || len(res) != 1 || res[0].ID != pubID || res[0].MembersCount != 2 || !res[0].IsMember {
		t.Fatalf("search golang: %+v %v", res, err)
	}
	if res, _ = e.convs.SearchPublicGroups(ctx, viewer, "go", 10, 0); len(res) != 0 {
		t.Fatal("short query must return nothing")
	}
	if res, _ = e.convs.SearchPublicGroups(ctx, viewer, "беседка", 10, 0); len(res) != 1 {
		t.Fatalf("case-insensitive cyrillic search: %+v", res)
	}
	// Спецсимволы LIKE не работают как шаблон.
	if res, _ = e.convs.SearchPublicGroups(ctx, viewer, "%%%", 10, 0); len(res) != 0 {
		t.Fatalf("wildcards must be escaped: %+v", res)
	}
	if res, _ = e.convs.SearchPublicGroups(ctx, viewer, "100%_", 10, 0); len(res) != 1 {
		t.Fatalf("literal %% and _ must match literally: %+v", res)
	}
}

func TestApproveRespectsGroupLimit(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, applicant := uuid.New(), uuid.New()
	res := createGroup(t, e, admin, "")
	conv := res.Conversation.ID

	req, err := e.joins.RequestJoin(ctx, applicant, conv, *res.InviteToken)
	if err != nil {
		t.Fatal(err)
	}
	fillMembers(t, e, conv, 4999) // вместе с admin 5000
	if err = e.joins.Approve(ctx, admin, conv, req.ID); !errors.Is(err, apperror.ErrGroupFull) {
		t.Fatalf("approve in a full group: %v", err)
	}
	if st := count(t, e.pool, `SELECT COUNT(*) FROM conversation_join_requests WHERE id = $1 AND request_status = 'pending'`, req.ID); st != 1 {
		t.Fatal("failed approve must leave the request pending")
	}
}

func TestPendingJoinRequestsLimit(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin := uuid.New()
	res := createGroup(t, e, admin, "")
	conv, token := res.Conversation.ID, *res.InviteToken

	if _, err := e.pool.Exec(ctx,
		`INSERT INTO conversation_join_requests (conversation_id, user_id) SELECT $1, gen_random_uuid() FROM generate_series(1, 1000)`, conv); err != nil {
		t.Fatal(err)
	}
	if _, err := e.joins.RequestJoin(ctx, uuid.New(), conv, token); !errors.Is(err, apperror.ErrJoinRequestsLimit) {
		t.Fatalf("request over the limit: %v", err)
	}

	// Обработанные заявки лимит освобождают.
	if _, err := e.pool.Exec(ctx,
		`UPDATE conversation_join_requests SET request_status = 'rejected'
		 WHERE id = (SELECT id FROM conversation_join_requests WHERE conversation_id = $1 LIMIT 1)`, conv); err != nil {
		t.Fatal(err)
	}
	if _, err := e.joins.RequestJoin(ctx, uuid.New(), conv, token); err != nil {
		t.Fatalf("request after a slot freed up: %v", err)
	}
}
