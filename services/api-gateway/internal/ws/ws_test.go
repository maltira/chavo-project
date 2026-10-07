package ws_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authv1 "github.com/maltira/chavo-project-backend/proto/gen/go/auth/v1"
	userv1 "github.com/maltira/chavo-project-backend/proto/gen/go/user/v1"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/hub"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/middleware"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/presence"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/profile"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/ws"
)

const origin = "http://localhost:3000"

type session struct{ userID, sid string }

type fakeAuth struct {
	authv1.AuthServiceClient
	sessions map[string]session
	err      error
}

func (f *fakeAuth) ResolveSession(_ context.Context, req *authv1.ResolveSessionRequest, _ ...grpc.CallOption) (*authv1.ResolveSessionResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	s, ok := f.sessions[req.GetRefreshToken()]
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "")
	}
	return &authv1.ResolveSessionResponse{UserId: s.userID, SessionId: s.sid}, nil
}

type fakeUsers struct {
	userv1.UserInternalServiceClient
	profiles map[string]bool
}

func (f *fakeUsers) UserExists(_ context.Context, req *userv1.UserExistsRequest, _ ...grpc.CallOption) (*userv1.UserExistsResponse, error) {
	return &userv1.UserExistsResponse{Exists: f.profiles[req.GetUserId()]}, nil
}

type noCache struct{}

func (noCache) Has(context.Context, string) (bool, error) { return false, nil }
func (noCache) Mark(context.Context, string) error        { return nil }
func (noCache) Forget(context.Context, string) error      { return nil }

type fakeRegistry struct {
	mu    sync.Mutex
	conns map[string]string
}

func (r *fakeRegistry) Add(_ context.Context, userID, connID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.conns[connID] = userID
	return nil
}

func (r *fakeRegistry) Remove(_ context.Context, _, connID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.conns, connID)
	return nil
}

func (r *fakeRegistry) len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.conns)
}

type presenceCall struct {
	kind string
	flag bool
	ids  []string
}

type fakePresence struct {
	mu    sync.Mutex
	calls []presenceCall
	err   error
}

func (p *fakePresence) record(c presenceCall) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, c)
}

func (p *fakePresence) Connected(_ context.Context, _ *hub.Conn, first bool) {
	p.record(presenceCall{kind: "connected", flag: first})
}
func (p *fakePresence) Disconnected(_ *hub.Conn, last bool) {
	p.record(presenceCall{kind: "disconnected", flag: last})
}
func (p *fakePresence) Subscribe(_ context.Context, c *hub.Conn, ids []string) error {
	p.record(presenceCall{kind: "subscribe", ids: ids})
	if p.err == nil {
		c.Enqueue([]byte(`{"type":"presence.snapshot"}`))
	}
	return p.err
}
func (p *fakePresence) Unsubscribe(_ *hub.Conn, ids []string) error {
	p.record(presenceCall{kind: "unsubscribe", ids: ids})
	return p.err
}

func (p *fakePresence) snapshot() []presenceCall {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]presenceCall(nil), p.calls...)
}

type env struct {
	url      string
	auth     *fakeAuth
	hub      *hub.Hub
	reg      *fakeRegistry
	presence *fakePresence
}

func newEnv(t *testing.T, opts ws.Options) *env {
	t.Helper()
	gin.SetMode(gin.TestMode)
	e := &env{
		auth: &fakeAuth{sessions: map[string]session{
			"tab":     {"u1", "s1"},
			"phone":   {"u1", "s2"},
			"noprof":  {"u2", "s3"},
			"another": {"u3", "s4"},
		}},
		reg:      &fakeRegistry{conns: map[string]string{}},
		presence: &fakePresence{},
	}
	log := zap.NewNop()
	e.hub = hub.New(e.reg, log)
	users := &fakeUsers{profiles: map[string]bool{"u1": true, "u3": true}}
	h, err := ws.New(ws.Deps{
		Auth: e.auth, Profiles: profile.NewGate(noCache{}, users, log), Hub: e.hub, Presence: e.presence,
		Origin: origin + "/", Options: opts, Log: log,
	})
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.Use(middleware.Trace())
	r.GET("/ws", h.Handle)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	e.url = "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	return e
}

func (e *env) dial(t *testing.T, token, org string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	hdr := http.Header{}
	if org != "" {
		hdr.Set("Origin", org)
	}
	if token != "" {
		hdr.Set("Cookie", "refresh_token="+token)
	}
	conn, resp, err := websocket.DefaultDialer.Dial(e.url, hdr)
	if conn != nil {
		t.Cleanup(func() { _ = conn.Close() })
	}
	return conn, resp, err
}

func read(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("decode %q: %v", data, err)
	}
	return m
}

// closeCode читает до кадра закрытия и возвращает его код.
func closeCode(t *testing.T, conn *websocket.Conn) int {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			var ce *websocket.CloseError
			if errors.As(err, &ce) {
				return ce.Code
			}
			t.Fatalf("expected close frame, got %v", err)
		}
	}
}

func connect(t *testing.T, e *env, token string) (*websocket.Conn, string) {
	t.Helper()
	conn, _, err := e.dial(t, token, origin)
	if err != nil {
		t.Fatal(err)
	}
	m := read(t, conn)
	if m["type"] != "ready" {
		t.Fatalf("expected ready, got %v", m)
	}
	return conn, m["connection_id"].(string)
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestOriginChecked(t *testing.T) {
	e := newEnv(t, ws.DefaultOptions())
	for _, org := range []string{"", "http://evil.example", "https://localhost:3000"} {
		_, resp, err := e.dial(t, "tab", org)
		if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
			t.Fatalf("origin %q: err=%v resp=%v", org, err, resp)
		}
	}
}

func TestHandshakeRejections(t *testing.T) {
	e := newEnv(t, ws.DefaultOptions())
	cases := []struct {
		name, token, reason string
		code                int
		authErr             error
	}{
		{"no cookie", "", "UNAUTHORIZED", hub.CloseUnauthorized, nil},
		{"unknown session", "stale", "UNAUTHORIZED", hub.CloseUnauthorized, nil},
		{"no profile", "noprof", "PROFILE_REQUIRED", hub.CloseProfileRequired, nil},
		{"auth down", "tab", "SERVICE_UNAVAILABLE", hub.CloseTryAgainLater, status.Error(codes.Unavailable, "")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e.auth.err = c.authErr
			conn, _, err := e.dial(t, c.token, origin)
			if err != nil {
				t.Fatal(err)
			}
			m := read(t, conn)
			if m["type"] != "error" || m["reason"] != c.reason {
				t.Fatalf("got %v", m)
			}
			if code := closeCode(t, conn); code != c.code {
				t.Fatalf("close code %d, want %d", code, c.code)
			}
		})
	}
	e.auth.err = nil
	if e.hub.Len() != 0 || e.reg.len() != 0 {
		t.Fatal("rejected connection registered")
	}
}

func TestConnectionLifecycle(t *testing.T) {
	e := newEnv(t, ws.DefaultOptions())
	conn, id := connect(t, e, "tab")
	if !strings.HasPrefix(id, "s1:") {
		t.Fatalf("connection_id %q not prefixed with sid", id)
	}
	if e.reg.len() != 1 || !e.hub.Online("u1") {
		t.Fatal("connection not registered")
	}

	for _, tc := range []struct{ in, typ, reason string }{
		{`{"type":"ping"}`, "pong", ""},
		{`{"type":`, "error", "INVALID_MESSAGE"},
		{`{"type":"subscribe-everything"}`, "error", "UNKNOWN_TYPE"},
	} {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(tc.in)); err != nil {
			t.Fatal(err)
		}
		m := read(t, conn)
		if m["type"] != tc.typ || (tc.reason != "" && m["reason"] != tc.reason) {
			t.Fatalf("%s → %v", tc.in, m)
		}
	}

	e.hub.SendToUsers([]string{"u1"}, []byte(`{"type":"message.created"}`))
	if m := read(t, conn); m["type"] != "message.created" {
		t.Fatalf("got %v", m)
	}

	_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	waitFor(t, "unregister", func() bool { return e.hub.Len() == 0 && e.reg.len() == 0 })
}

func TestCloseSIDKeepsOtherSessions(t *testing.T) {
	e := newEnv(t, ws.DefaultOptions())
	tab1, _ := connect(t, e, "tab")
	tab2, _ := connect(t, e, "tab")
	phone, _ := connect(t, e, "phone")

	e.hub.CloseSID("s1", []byte(`{"type":"session.revoked"}`), hub.CloseSessionRevoked, "session revoked")
	for _, c := range []*websocket.Conn{tab1, tab2} {
		if m := read(t, c); m["type"] != "session.revoked" {
			t.Fatalf("got %v", m)
		}
		if code := closeCode(t, c); code != hub.CloseSessionRevoked {
			t.Fatalf("close code %d", code)
		}
	}
	waitFor(t, "tabs unregistered", func() bool { return e.hub.Len() == 1 })

	e.hub.SendToUsers([]string{"u1"}, []byte(`{"type":"still-here"}`))
	if m := read(t, phone); m["type"] != "still-here" {
		t.Fatalf("phone got %v", m)
	}
}

func TestMessageTooBig(t *testing.T) {
	e := newEnv(t, ws.DefaultOptions())
	conn, _ := connect(t, e, "tab")
	_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"`+strings.Repeat("a", 5000)+`"}`))
	if code := closeCode(t, conn); code != websocket.CloseMessageTooBig {
		t.Fatalf("close code %d", code)
	}
	waitFor(t, "unregister", func() bool { return e.hub.Len() == 0 })
}

func TestDeadClientDropped(t *testing.T) {
	opts := ws.DefaultOptions()
	opts.PingInterval, opts.PongWait = 50*time.Millisecond, 200*time.Millisecond
	e := newEnv(t, opts)

	// Клиент, который читает, отвечает на ping и остаётся подключённым.
	alive, _ := connect(t, e, "another")
	go func() {
		for {
			if _, _, err := alive.ReadMessage(); err != nil {
				return
			}
		}
	}()

	// Клиент, который перестал читать, не отвечает pong и отключается по таймауту.
	connect(t, e, "tab")
	waitFor(t, "dead client dropped", func() bool { return !e.hub.Online("u1") })
	time.Sleep(300 * time.Millisecond)
	if !e.hub.Online("u3") {
		t.Fatal("live client dropped")
	}
}

func TestShutdownClosesConnections(t *testing.T) {
	e := newEnv(t, ws.DefaultOptions())
	conn, _ := connect(t, e, "tab")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- e.hub.Shutdown(ctx) }()

	if code := closeCode(t, conn); code != hub.CloseGoingAway {
		t.Fatalf("close code %d", code)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if e.reg.len() != 0 {
		t.Fatal("registry not cleaned")
	}
}

func TestPresenceCommands(t *testing.T) {
	e := newEnv(t, ws.DefaultOptions())
	tab1, _ := connect(t, e, "tab")
	tab2, _ := connect(t, e, "tab")

	send := func(c *websocket.Conn, msg string) {
		if err := c.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			t.Fatal(err)
		}
	}
	send(tab1, `{"type":"presence.subscribe","user_ids":["a","b"]}`)
	if m := read(t, tab1); m["type"] != "presence.snapshot" {
		t.Fatalf("got %v", m)
	}
	send(tab1, `{"type":"presence.unsubscribe","user_ids":["a"]}`)
	send(tab1, `{"type":"ping"}`)
	if m := read(t, tab1); m["type"] != "pong" {
		t.Fatalf("unsubscribe must not answer, got %v", m)
	}

	for err, reason := range map[error]string{
		presence.ErrInvalidIDs: "INVALID_MESSAGE",
		presence.ErrTooMany:    "PRESENCE_LIMIT",
		hub.ErrWatchLimit:      "PRESENCE_LIMIT",
		errors.New("down"):     "SERVICE_UNAVAILABLE",
	} {
		e.presence.mu.Lock()
		e.presence.err = err
		e.presence.mu.Unlock()
		send(tab1, `{"type":"presence.subscribe","user_ids":["a"]}`)
		if m := read(t, tab1); m["type"] != "error" || m["reason"] != reason {
			t.Fatalf("%v: got %v", err, m)
		}
	}

	_ = tab1.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	waitFor(t, "first tab closed", func() bool { return e.hub.Len() == 1 })
	_ = tab2.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	waitFor(t, "second tab closed", func() bool { return e.hub.Len() == 0 })

	var lifecycle []presenceCall
	for _, c := range e.presence.snapshot() {
		if c.kind == "connected" || c.kind == "disconnected" {
			lifecycle = append(lifecycle, c)
		}
	}
	want := []presenceCall{{"connected", true, nil}, {"connected", false, nil}, {"disconnected", false, nil}, {"disconnected", true, nil}}
	if len(lifecycle) != len(want) {
		t.Fatalf("lifecycle %+v", lifecycle)
	}
	for i := range want {
		if lifecycle[i].kind != want[i].kind || lifecycle[i].flag != want[i].flag {
			t.Fatalf("lifecycle %+v, want %+v", lifecycle, want)
		}
	}
}
