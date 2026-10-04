package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/handler"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/router"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/testutil"
	"github.com/maltira/chavo-project-backend/services/conversation-service/pkg/crypto"
)

type allowAllUsers struct{}

func (allowAllUsers) CheckMessagingAllowed(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (allowAllUsers) UserExists(context.Context, uuid.UUID) (bool, error)               { return true, nil }
func (allowAllUsers) GroupInviteAllowed(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}

func newServer(t *testing.T, ping func(context.Context) error) http.Handler {
	t.Helper()
	pool := testutil.Pool(t, "router_test")
	cipher, _ := crypto.NewCipher(bytes.Repeat([]byte{5}, 32))
	db := repository.NewDB(pool)
	cr, mr, or := repository.NewConversationRepository(), repository.NewMessageRepository(), repository.NewOutboxRepository()
	br, jr := repository.NewBanRepository(), repository.NewJoinRequestRepository()
	users := allowAllUsers{}
	log := zap.NewNop()

	return router.SetupRouter(
		handler.NewConversationHandler(service.NewConversationService(db, cr, cipher), log),
		handler.NewGroupHandler(service.NewGroupService(db, cr, or, users, br), log),
		handler.NewJoinHandler(service.NewJoinService(db, cr, jr, or, br), log),
		handler.NewMessageHandler(service.NewMessageService(db, cr, mr, or, users, cipher, log), log),
		ping,
	)
}

type resp struct {
	Code   int
	Body   map[string]any
	Raw    string
	Reason string
}

func call(t *testing.T, h http.Handler, method, path string, user *uuid.UUID, body any) resp {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if user != nil {
		req.Header.Set("X-User-ID", user.String())
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	r := resp{Code: rec.Code, Raw: rec.Body.String()}
	_ = json.Unmarshal(rec.Body.Bytes(), &r.Body)
	r.Reason, _ = r.Body["reason"].(string)
	return r
}

func id(u uuid.UUID) *uuid.UUID { return &u }

func TestHealth(t *testing.T) {
	ok := newServer(t, func(context.Context) error { return nil })
	if r := call(t, ok, http.MethodGet, "/health", nil, nil); r.Code != http.StatusOK || r.Body["status"] != "ok" {
		t.Fatalf("healthy: %d %s", r.Code, r.Raw)
	}
	down := newServer(t, func(context.Context) error { return errors.New("db down") })
	if r := call(t, down, http.MethodGet, "/health", nil, nil); r.Code != http.StatusServiceUnavailable {
		t.Fatalf("db down: %d %s", r.Code, r.Raw)
	}
}

func TestIdentityAndParamValidation(t *testing.T) {
	h := newServer(t, func(context.Context) error { return nil })
	u := uuid.New()

	if r := call(t, h, http.MethodGet, "/conversations", nil, nil); r.Code != http.StatusBadRequest {
		t.Fatalf("no X-User-ID: %d %s", r.Code, r.Raw)
	}
	if r := call(t, h, http.MethodGet, "/conversations/not-a-uuid", id(u), nil); r.Code != http.StatusBadRequest {
		t.Fatalf("bad conversation id: %d %s", r.Code, r.Raw)
	}
	if r := call(t, h, http.MethodGet, "/messages?conversation_id=nope", id(u), nil); r.Code != http.StatusBadRequest {
		t.Fatalf("bad conversation_id: %d %s", r.Code, r.Raw)
	}
	if r := call(t, h, http.MethodGet, "/messages?conversation_id="+uuid.NewString()+"&before=nope", id(u), nil); r.Code != http.StatusBadRequest {
		t.Fatalf("bad before: %d %s", r.Code, r.Raw)
	}
	if r := call(t, h, http.MethodPost, "/messages", id(u), map[string]any{"content": ""}); r.Code != http.StatusBadRequest {
		t.Fatalf("empty content: %d %s", r.Code, r.Raw)
	}
	if r := call(t, h, http.MethodGet, "/conversations/"+uuid.NewString(), id(u), nil); r.Code != http.StatusNotFound {
		t.Fatalf("unknown conversation: %d %s", r.Code, r.Raw)
	}
}

func TestMessageFlowStatusesAndPagination(t *testing.T) {
	h := newServer(t, func(context.Context) error { return nil })
	a, b, outsider := uuid.New(), uuid.New(), uuid.New()

	first := call(t, h, http.MethodPost, "/messages", id(a), map[string]any{"recipient_id": b, "content": "m1"})
	if first.Code != http.StatusCreated {
		t.Fatalf("send: %d %s", first.Code, first.Raw)
	}
	conv := first.Body["conversation_id"].(string)
	for _, text := range []string{"m2", "m3"} {
		if r := call(t, h, http.MethodPost, "/messages", id(a), map[string]any{"conversation_id": conv, "content": text}); r.Code != http.StatusCreated {
			t.Fatalf("send %s: %d %s", text, r.Code, r.Raw)
		}
	}

	page1 := call(t, h, http.MethodGet, "/messages?conversation_id="+conv+"&limit=2", id(b), nil)
	next, _ := page1.Body["next_before"].(string)
	if page1.Code != http.StatusOK || len(page1.Body["items"].([]any)) != 2 || next == "" {
		t.Fatalf("page 1: %d %s", page1.Code, page1.Raw)
	}
	page2 := call(t, h, http.MethodGet, "/messages?conversation_id="+conv+"&limit=2&before="+next, id(b), nil)
	if page2.Code != http.StatusOK || len(page2.Body["items"].([]any)) != 1 || page2.Body["next_before"] != nil {
		t.Fatalf("page 2: %d %s", page2.Code, page2.Raw)
	}

	if r := call(t, h, http.MethodGet, "/messages?conversation_id="+conv, id(outsider), nil); r.Code != http.StatusForbidden || r.Reason != apperror.Reason(apperror.ErrNotMember) {
		t.Fatalf("outsider: %d %s", r.Code, r.Raw)
	}
}

func TestGroupErrorsAndBanOverHTTP(t *testing.T) {
	h := newServer(t, func(context.Context) error { return nil })
	admin, member := uuid.New(), uuid.New()

	created := call(t, h, http.MethodPost, "/conversations", id(admin), map[string]any{"name": "g", "visibility": "public", "member_ids": []uuid.UUID{member}})
	if created.Code != http.StatusCreated {
		t.Fatalf("create group: %d %s", created.Code, created.Raw)
	}
	conv := created.Body["id"].(string)

	if r := call(t, h, http.MethodDelete, "/conversations/"+conv+"/members/me", id(admin), nil); r.Code != http.StatusConflict || r.Reason != "LAST_ADMIN" {
		t.Fatalf("last admin leaves: %d %s", r.Code, r.Raw)
	}
	if r := call(t, h, http.MethodDelete, "/conversations/"+conv+"/members/"+admin.String()+"?ban=true", id(admin), nil); r.Code != http.StatusBadRequest {
		t.Fatalf("ban self: %d %s", r.Code, r.Raw)
	}
	if r := call(t, h, http.MethodGet, "/conversations/"+conv+"/bans", id(member), nil); r.Code != http.StatusForbidden {
		t.Fatalf("member lists bans: %d %s", r.Code, r.Raw)
	}

	if r := call(t, h, http.MethodDelete, "/conversations/"+conv+"/members/"+member.String()+"?ban=true", id(admin), nil); r.Code != http.StatusOK {
		t.Fatalf("kick with ban: %d %s", r.Code, r.Raw)
	}
	if r := call(t, h, http.MethodPost, "/conversations/"+conv+"/join", id(member), nil); r.Code != http.StatusForbidden || r.Reason != "USER_BANNED" {
		t.Fatalf("banned join: %d %s", r.Code, r.Raw)
	}
	if bans := call(t, h, http.MethodGet, "/conversations/"+conv+"/bans", id(admin), nil); bans.Code != http.StatusOK || len(bans.Body["items"].([]any)) != 1 {
		t.Fatalf("bans: %d %s", bans.Code, bans.Raw)
	}
	if r := call(t, h, http.MethodDelete, "/conversations/"+conv+"/bans/"+member.String(), id(admin), nil); r.Code != http.StatusOK {
		t.Fatalf("unban: %d %s", r.Code, r.Raw)
	}
	if r := call(t, h, http.MethodPost, "/conversations/"+conv+"/join", id(member), nil); r.Code != http.StatusOK {
		t.Fatalf("join after unban: %d %s", r.Code, r.Raw)
	}
}
