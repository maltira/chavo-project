package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/hub"
)

type sent struct {
	users []string
	msg   []byte
}

type closed struct {
	key   string
	final []byte
	code  int
}

type watcherSend struct {
	owner   string
	msg     []byte
	exclude map[string]bool
}

type fakeSink struct {
	sent       []sent
	watchers   []watcherSend
	closedSIDs []closed
	closedUser []closed
}

func (s *fakeSink) SendToWatchersExcept(owner string, msg []byte, exclude map[string]bool) {
	s.watchers = append(s.watchers, watcherSend{owner, msg, exclude})
}

func (s *fakeSink) SendToUsers(ids []string, msg []byte) { s.sent = append(s.sent, sent{ids, msg}) }
func (s *fakeSink) CloseSID(sid string, final []byte, code int, _ string) {
	s.closedSIDs = append(s.closedSIDs, closed{sid, final, code})
}
func (s *fakeSink) CloseUser(id string, final []byte, code int, _ string) {
	s.closedUser = append(s.closedUser, closed{id, final, code})
}

type fakeProfiles struct {
	deleted []string
	err     error
}

func (p *fakeProfiles) Deleted(_ context.Context, id string) error {
	if p.err != nil {
		return p.err
	}
	p.deleted = append(p.deleted, id)
	return nil
}

var now = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

func newRouter() (*Router, *fakeSink, *fakeProfiles) {
	sink, profiles := &fakeSink{}, &fakeProfiles{}
	r := NewRouter(sink, nil, profiles, DefaultOptions(), zap.NewNop())
	r.now = func() time.Time { return now }
	return r, sink, profiles
}

var seq int

func event(typ string, payload any, at time.Time) []byte {
	seq++
	b, _ := json.Marshal(map[string]any{
		"event_id": fmt.Sprintf("ev-%d", seq), "event_type": typ, "occurred_at": at, "payload": payload,
	})
	return b
}

func decode(t *testing.T, msg []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(msg, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestRecipients(t *testing.T) {
	cases := []struct {
		typ     string
		payload map[string]any
		want    []string
	}{
		{TypeMessageCreated, map[string]any{"member_ids": []string{"a", "b"}, "content": "hi"}, []string{"a", "b"}},
		{TypeMessageUpdated, map[string]any{"member_ids": []string{"a"}}, []string{"a"}},
		{TypeMessageDeleted, map[string]any{"member_ids": []string{"a"}}, []string{"a"}},
		{TypeConversationCreated, map[string]any{"member_ids": []string{"a", "b"}}, []string{"a", "b"}},
		{TypeConversationUpdated, map[string]any{"member_ids": []string{"a"}}, []string{"a"}},
		{TypeConversationDeleted, map[string]any{"member_ids": []string{"a"}}, []string{"a"}},
		{TypeMemberAdded, map[string]any{"member_ids": []string{"a", "new"}}, []string{"a", "new"}},
		{TypeMemberRemoved, map[string]any{"member_ids": []string{"a", "gone"}, "user_id": "gone"}, []string{"a", "gone"}},
		{TypeJoinRequestApproved, map[string]any{"member_ids": []string{"a"}}, []string{"a"}},
		{TypeJoinRequestCreated, map[string]any{"admin_ids": []string{"adm"}, "user_id": "x"}, []string{"adm"}},
		{TypeMessageRead, map[string]any{"author_ids": []string{"a1", "a2"}, "reader_id": "r"}, []string{"a1", "a2", "r"}},
		{TypeMessageRead, map[string]any{"author_ids": []string{}, "reader_id": "r"}, []string{"r"}},
		{TypeUserBlocked, map[string]any{"blocker_id": "x", "blocked_id": "y"}, []string{"x", "y"}},
		{TypeUserUnblocked, map[string]any{"blocker_id": "x", "blocked_id": "y"}, []string{"x", "y"}},
		{TypeMessageCreated, map[string]any{"member_ids": []string{"a", "a", ""}}, []string{"a"}},
	}
	for _, c := range cases {
		t.Run(c.typ, func(t *testing.T) {
			r, sink, _ := newRouter()
			if err := r.Handle(context.Background(), event(c.typ, c.payload, now)); err != nil {
				t.Fatal(err)
			}
			if len(sink.sent) != 1 || !slices.Equal(sink.sent[0].users, c.want) {
				t.Fatalf("sent %+v, want %v", sink.sent, c.want)
			}
		})
	}
}

func TestFrameHidesRoutingFields(t *testing.T) {
	r, sink, _ := newRouter()
	payload := map[string]any{
		"conversation_id": "c", "member_ids": []string{"a"}, "admin_ids": []string{"x"}, "author_ids": []string{"y"},
		"content_enc": "c2VjcmV0", "content": "hi",
	}
	if err := r.Handle(context.Background(), event(TypeMessageCreated, payload, now)); err != nil {
		t.Fatal(err)
	}
	m := decode(t, sink.sent[0].msg)
	if m["type"] != TypeMessageCreated || m["event_id"] == "" || m["occurred_at"] == "" {
		t.Fatalf("envelope %v", m)
	}
	data := m["data"].(map[string]any)
	for _, f := range hiddenFields {
		if _, ok := data[f]; ok {
			t.Fatalf("%s leaked to client: %v", f, data)
		}
	}
	if data["conversation_id"] != "c" || data["content"] != "hi" {
		t.Fatalf("data %v", data)
	}
}

func TestSessionRevoked(t *testing.T) {
	r, sink, _ := newRouter()
	// Даже старое событие закрывает сессию: Kafka могла отставать, пока соединение было живо.
	ev := event(TypeSessionRevoked, map[string]any{"session_id": "s1", "user_id": "u", "reason": "remote_logout"}, now.Add(-time.Hour))
	if err := r.Handle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(sink.closedSIDs) != 1 || sink.closedSIDs[0].key != "s1" || sink.closedSIDs[0].code != hub.CloseSessionRevoked {
		t.Fatalf("closed %+v", sink.closedSIDs)
	}
	if m := decode(t, sink.closedSIDs[0].final); m["type"] != TypeSessionRevoked {
		t.Fatalf("final %v", m)
	}
	if len(sink.sent) != 0 || len(sink.closedUser) != 0 {
		t.Fatal("unexpected delivery")
	}
}

func TestAccountDeleted(t *testing.T) {
	r, sink, profiles := newRouter()
	ev := event(TypeAccountDeleted, map[string]any{"user_id": "u"}, now.Add(-time.Hour))

	profiles.err = errors.New("redis down")
	if err := r.Handle(context.Background(), ev); err == nil || errors.Is(err, ErrMalformed) {
		t.Fatalf("expected retryable error, got %v", err)
	}
	if len(sink.closedUser) != 0 {
		t.Fatal("connections closed before profile cache cleared")
	}

	// Неудачная попытка не запоминается: повтор того же события проходит.
	profiles.err = nil
	if err := r.Handle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(profiles.deleted, []string{"u"}) || len(sink.closedUser) != 1 || sink.closedUser[0].key != "u" {
		t.Fatalf("deleted %v closed %+v", profiles.deleted, sink.closedUser)
	}
}

func TestStaleEventsSkipped(t *testing.T) {
	r, sink, _ := newRouter()
	payload := map[string]any{"member_ids": []string{"a"}}
	if err := r.Handle(context.Background(), event(TypeMessageCreated, payload, now.Add(-2*time.Minute))); err != nil {
		t.Fatal(err)
	}
	if err := r.Handle(context.Background(), event(TypeMessageCreated, payload, now.Add(-30*time.Second))); err != nil {
		t.Fatal(err)
	}
	if len(sink.sent) != 1 {
		t.Fatalf("sent %d, want only the fresh event", len(sink.sent))
	}
}

func TestDuplicateDeliveredOnce(t *testing.T) {
	r, sink, _ := newRouter()
	ev := event(TypeMessageCreated, map[string]any{"member_ids": []string{"a"}}, now)
	for range 3 {
		if err := r.Handle(context.Background(), ev); err != nil {
			t.Fatal(err)
		}
	}
	if len(sink.sent) != 1 {
		t.Fatalf("sent %d times", len(sink.sent))
	}
}

func TestMalformedAndUnknown(t *testing.T) {
	r, sink, _ := newRouter()
	for _, raw := range []string{
		`not json`,
		`{"event_type":"message.created","payload":{}}`,
		`{"event_id":"x","event_type":"message.created"}`,
		`{"event_id":"y","event_type":"message.created","payload":[1]}`,
		`{"event_id":"z","event_type":"message.created","payload":{"member_ids":["a"]}}`, // нет occurred_at
		`{"event_id":"w","event_type":"session.revoked","occurred_at":"2026-10-07T12:00:00Z","payload":{}}`,
	} {
		if err := r.Handle(context.Background(), []byte(raw)); !errors.Is(err, ErrMalformed) {
			t.Fatalf("%s: %v", raw, err)
		}
	}
	if err := r.Handle(context.Background(), event("user.online", map[string]any{"user_id": "a"}, now)); err != nil {
		t.Fatal(err)
	}
	if len(sink.sent)+len(sink.closedSIDs)+len(sink.closedUser) != 0 {
		t.Fatal("unexpected delivery")
	}
}

func TestDedupEviction(t *testing.T) {
	d := newDedup(time.Minute, 2)
	d.add("a", now)
	d.add("b", now)
	d.add("c", now)
	if d.has("a", now) || !d.has("b", now) || !d.has("c", now) {
		t.Fatal("size limit not applied")
	}
	if d.has("b", now.Add(time.Minute)) {
		t.Fatal("ttl not applied")
	}
}

type fakePresence struct {
	pairs   [][2]string
	blocked map[string]map[string]bool
	err     error
}

func (p *fakePresence) DirectCreated(_ context.Context, a, b string) {
	p.pairs = append(p.pairs, [2]string{a, b})
}

func (p *fakePresence) Blocked(_ context.Context, owner string) (map[string]bool, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.blocked[owner], nil
}

func TestDirectCreatedWatchesPeers(t *testing.T) {
	sink, pres := &fakeSink{}, &fakePresence{}
	r := NewRouter(sink, pres, &fakeProfiles{}, DefaultOptions(), zap.NewNop())
	r.now = func() time.Time { return now }

	// Даже устаревшее событие связывает собеседников: доставка пропускается, наблюдение — нет.
	direct := map[string]any{"conversation_type": "direct", "member_ids": []string{"a", "b"}}
	if err := r.Handle(context.Background(), event(TypeConversationCreated, direct, now.Add(-time.Hour))); err != nil {
		t.Fatal(err)
	}
	group := map[string]any{"conversation_type": "group", "member_ids": []string{"a", "b"}}
	if err := r.Handle(context.Background(), event(TypeConversationCreated, group, now)); err != nil {
		t.Fatal(err)
	}
	if len(pres.pairs) != 1 || pres.pairs[0] != [2]string{"a", "b"} {
		t.Fatalf("pairs %v", pres.pairs)
	}
	if len(sink.sent) != 1 {
		t.Fatalf("sent %d, want only the fresh group event", len(sink.sent))
	}
}

func TestStatusUpdatedRouting(t *testing.T) {
	sink, pres := &fakeSink{}, &fakePresence{blocked: map[string]map[string]bool{"owner": {"enemy": true}}}
	r := NewRouter(sink, pres, &fakeProfiles{}, DefaultOptions(), zap.NewNop())
	r.now = func() time.Time { return now }

	exp := now.Add(time.Hour)
	ev := event(TypeUserStatusUpdated, map[string]any{
		"user_id": "owner", "status": map[string]any{
			"icon": "shine", "text": "Отдыхаю", "background": "#34D399", "foreground": "#064E3B", "expires_at": exp,
		},
	}, now)
	if err := r.Handle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	// Владельцу — во все его вкладки, наблюдателям — кроме заблокированных.
	if len(sink.sent) != 1 || !slices.Equal(sink.sent[0].users, []string{"owner"}) {
		t.Fatalf("owner delivery: %+v", sink.sent)
	}
	if len(sink.watchers) != 1 || sink.watchers[0].owner != "owner" || !sink.watchers[0].exclude["enemy"] {
		t.Fatalf("watchers delivery: %+v", sink.watchers)
	}
	m := decode(t, sink.watchers[0].msg)
	st := m["data"].(map[string]any)["status"].(map[string]any)
	if m["type"] != TypeUserStatusUpdated || st["icon"] != "shine" || st["background"] != "#34D399" || st["foreground"] != "#064E3B" || st["text"] != "Отдыхаю" {
		t.Fatalf("frame: %v", m)
	}

	// Удаление статуса — status: null.
	if err := r.Handle(context.Background(), event(TypeUserStatusUpdated, map[string]any{"user_id": "owner", "status": nil}, now)); err != nil {
		t.Fatal(err)
	}
	if m := decode(t, sink.sent[1].msg); m["data"].(map[string]any)["status"] != nil {
		t.Fatalf("cleared: %v", m)
	}

	// Иконка, убранная из набора, клиенту не показывается.
	if err := r.Handle(context.Background(), event(TypeUserStatusUpdated, map[string]any{
		"user_id": "owner", "status": map[string]any{"icon": "removed", "text": "x", "background": "#000000", "foreground": "#FFFFFF"},
	}, now)); err != nil {
		t.Fatal(err)
	}
	if m := decode(t, sink.sent[2].msg); m["data"].(map[string]any)["status"] != nil {
		t.Fatalf("unknown icon: %v", m)
	}

	// Старое событие не доставляется.
	before := len(sink.sent)
	if err := r.Handle(context.Background(), event(TypeUserStatusUpdated, map[string]any{"user_id": "owner", "status": nil}, now.Add(-2*time.Minute))); err != nil {
		t.Fatal(err)
	}
	if len(sink.sent) != before {
		t.Fatal("stale status delivered")
	}
}

func TestStatusUpdatedBlockedListError(t *testing.T) {
	sink, pres := &fakeSink{}, &fakePresence{err: errors.New("user-service down")}
	r := NewRouter(sink, pres, &fakeProfiles{}, DefaultOptions(), zap.NewNop())
	r.now = func() time.Time { return now }
	ev := event(TypeUserStatusUpdated, map[string]any{"user_id": "owner", "status": nil}, now)

	// Ошибка — повтор; до получения списка не доставляется никому, в том числе владельцу (иначе дубли).
	if err := r.Handle(context.Background(), ev); err == nil || errors.Is(err, ErrMalformed) {
		t.Fatalf("expected retryable error, got %v", err)
	}
	if len(sink.sent)+len(sink.watchers) != 0 {
		t.Fatal("delivered before blocked list was known")
	}
	pres.err = nil
	if err := r.Handle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(sink.sent) != 1 || len(sink.watchers) != 1 {
		t.Fatalf("after retry: %d %d", len(sink.sent), len(sink.watchers))
	}
}
