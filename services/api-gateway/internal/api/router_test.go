package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"

	authv1 "github.com/maltira/chavo-project-backend/proto/gen/go/auth/v1"
	conversationv1 "github.com/maltira/chavo-project-backend/proto/gen/go/conversation/v1"
	userv1 "github.com/maltira/chavo-project-backend/proto/gen/go/user/v1"
	"github.com/maltira/chavo-project-backend/proto/grpcx"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/api"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/auth"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/profile"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/reqctx"
)

const secret = "test-secret"

// ── фейковые зависимости ──────────────────────────────

type fakeSessions struct {
	mu     sync.Mutex
	owners map[string]string
	err    error
}

func (s *fakeSessions) Owner(_ context.Context, sid string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.owners[sid], s.err
}

type fakeCache struct {
	mu  sync.Mutex
	set map[string]bool
}

func (c *fakeCache) Has(_ context.Context, id string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.set[id], nil
}

func (c *fakeCache) Mark(_ context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.set[id] = true
	return nil
}

func (c *fakeCache) Forget(_ context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.set, id)
	return nil
}

// seenMD — metadata последнего вызова, как её увидел сервис.
type seenMD struct {
	mu sync.Mutex
	md metadata.MD
}

func (s *seenMD) record(ctx context.Context) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.mu.Lock()
	s.md = md
	s.mu.Unlock()
}

func (s *seenMD) get(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v := s.md.Get(key); len(v) > 0 {
		return v[0]
	}
	return ""
}

type fakeAuth struct {
	authv1.UnimplementedAuthServiceServer
}

func tokenPair() *authv1.TokenPair {
	return &authv1.TokenPair{
		AccessToken: "access", RefreshToken: "refresh-new",
		RefreshExpiresAt: timestamppb.New(time.Now().Add(time.Hour)),
	}
}

func (fakeAuth) VerifyOTP(context.Context, *authv1.VerifyOTPRequest) (*authv1.VerifyOTPResponse, error) {
	return &authv1.VerifyOTPResponse{Tokens: tokenPair()}, nil
}

func (fakeAuth) Refresh(_ context.Context, req *authv1.RefreshRequest) (*authv1.RefreshResponse, error) {
	if req.GetRefreshToken() != "refresh-old" {
		return nil, grpcx.Error(codes.Unauthenticated, "INVALID_TOKEN", "Сессия истекла")
	}
	return &authv1.RefreshResponse{Tokens: tokenPair()}, nil
}

func (fakeAuth) Logout(context.Context, *authv1.LogoutRequest) (*authv1.LogoutResponse, error) {
	return &authv1.LogoutResponse{}, nil
}

func (fakeAuth) ListSessions(context.Context, *authv1.ListSessionsRequest) (*authv1.ListSessionsResponse, error) {
	return &authv1.ListSessionsResponse{}, nil
}

type fakeUsers struct {
	userv1.UnimplementedUserServiceServer
	userv1.UnimplementedUserInternalServiceServer
	seen        *seenMD
	mu          sync.Mutex
	profiles    map[string]bool
	existsCalls atomic.Int32
}

func (u *fakeUsers) has(id string) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.profiles[id]
}

func (u *fakeUsers) CreateProfile(ctx context.Context, _ *userv1.CreateProfileRequest) (*userv1.CreateProfileResponse, error) {
	u.mu.Lock()
	u.profiles[grpcx.IncomingUserID(ctx)] = true
	u.mu.Unlock()
	return &userv1.CreateProfileResponse{}, nil
}

func (u *fakeUsers) GetMe(ctx context.Context, _ *userv1.GetMeRequest) (*userv1.GetMeResponse, error) {
	u.seen.record(ctx)
	id := grpcx.IncomingUserID(ctx)
	if !u.has(id) {
		return nil, grpcx.Error(codes.NotFound, "PROFILE_NOT_FOUND", "Профиль не найден")
	}
	return &userv1.GetMeResponse{Profile: &userv1.Profile{UserId: id, Username: "me"}}, nil
}

func (u *fakeUsers) SearchProfiles(context.Context, *userv1.SearchProfilesRequest) (*userv1.SearchProfilesResponse, error) {
	return &userv1.SearchProfilesResponse{}, nil
}

func (u *fakeUsers) UserExists(_ context.Context, req *userv1.UserExistsRequest) (*userv1.UserExistsResponse, error) {
	u.existsCalls.Add(1)
	return &userv1.UserExistsResponse{Exists: u.has(req.GetUserId())}, nil
}

type fakeConversations struct {
	conversationv1.UnimplementedConversationServiceServer
	sendErr error
}

func (fakeConversations) ListConversations(context.Context, *conversationv1.ListConversationsRequest) (*conversationv1.ListConversationsResponse, error) {
	return &conversationv1.ListConversationsResponse{Limit: 30}, nil
}

func (f fakeConversations) SendMessage(_ context.Context, req *conversationv1.SendMessageRequest) (*conversationv1.SendMessageResponse, error) {
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	return &conversationv1.SendMessageResponse{Message: &conversationv1.Message{
		Id: uuid.NewString(), ConversationId: req.GetConversationId(), Content: &req.Content, CreatedAt: timestamppb.Now(),
	}}, nil
}

// ── окружение теста ───────────────────────────────────

type env struct {
	router   http.Handler
	sessions *fakeSessions
	cache    *fakeCache
	users    *fakeUsers
	seen     *seenMD
	convs    *fakeConversations
}

func newEnv(t *testing.T) *env {
	t.Helper()
	gin.SetMode(gin.TestMode)

	e := &env{
		sessions: &fakeSessions{owners: map[string]string{}},
		cache:    &fakeCache{set: map[string]bool{}},
		seen:     &seenMD{},
		convs:    &fakeConversations{},
	}
	e.users = &fakeUsers{seen: e.seen, profiles: map[string]bool{}}

	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	authv1.RegisterAuthServiceServer(srv, fakeAuth{})
	userv1.RegisterUserServiceServer(srv, e.users)
	userv1.RegisterUserInternalServiceServer(srv, e.users)
	conversationv1.RegisterConversationServiceServer(srv, e.convs)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(reqctx.ClientInterceptor(2*time.Second)),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	log := zap.NewNop()
	router, err := api.NewRouter(api.Deps{
		Auth:           authv1.NewAuthServiceClient(conn),
		Users:          userv1.NewUserServiceClient(conn),
		Conversations:  conversationv1.NewConversationServiceClient(conn),
		Verifier:       auth.NewVerifier(secret, e.sessions),
		Profiles:       profile.NewGate(e.cache, userv1.NewUserInternalServiceClient(conn), log),
		TrustedProxies: []string{"10.0.0.0/8"},
		Log:            log,
	})
	if err != nil {
		t.Fatal(err)
	}
	e.router = router
	return e
}

// login создаёт живую сессию и возвращает access token; withProfile — профиль уже есть.
func (e *env) login(t *testing.T, withProfile bool) (userID, token string) {
	t.Helper()
	userID, sid := uuid.NewString(), uuid.NewString()
	e.sessions.mu.Lock()
	e.sessions.owners[sid] = userID
	e.sessions.mu.Unlock()
	if withProfile {
		e.users.mu.Lock()
		e.users.profiles[userID] = true
		e.users.mu.Unlock()
	}
	return userID, sign(t, secret, userID, sid, time.Now().Add(time.Minute))
}

func sign(t *testing.T, key, sub, sid string, exp time.Time) string {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": sub, "sid": sid, "iat": time.Now().Unix(), "exp": exp.Unix(),
	}).SignedString([]byte(key))
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func (e *env) do(method, path, token, body string, hdr ...string) *httptest.ResponseRecorder {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		r.Header.Set(hdr[i], hdr[i+1])
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, r)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode %q: %v", w.Body.String(), err)
	}
	return m
}

func expect(t *testing.T, w *httptest.ResponseRecorder, status int, reason string) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d, want %d; body %s", w.Code, status, w.Body.String())
	}
	if reason != "" {
		if got := decode(t, w)["reason"]; got != reason {
			t.Fatalf("reason = %v, want %s", got, reason)
		}
	}
}

// ── тесты ─────────────────────────────────────────────

func TestAuthenticationRejects(t *testing.T) {
	e := newEnv(t)
	userID, sid := uuid.NewString(), uuid.NewString()
	e.sessions.owners[sid] = userID

	revokedSID := uuid.NewString()
	foreignSID := uuid.NewString()
	e.sessions.owners[foreignSID] = uuid.NewString()

	none := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"sub": userID, "sid": sid, "exp": time.Now().Add(time.Minute).Unix()})
	noneTok, _ := none.SignedString(jwt.UnsafeAllowNoneSignatureType)

	cases := map[string]string{
		"no token":        "",
		"garbage":         "not-a-jwt",
		"expired":         sign(t, secret, userID, sid, time.Now().Add(-time.Minute)),
		"wrong secret":    sign(t, "other", userID, sid, time.Now().Add(time.Minute)),
		"alg none":        noneTok,
		"revoked session": sign(t, secret, userID, revokedSID, time.Now().Add(time.Minute)),
		"foreign session": sign(t, secret, userID, foreignSID, time.Now().Add(time.Minute)),
		"sub not uuid":    sign(t, secret, "admin", sid, time.Now().Add(time.Minute)),
	}
	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			expect(t, e.do(http.MethodGet, "/api/users/me", tok, ""), http.StatusUnauthorized, "UNAUTHORIZED")
		})
	}

	t.Run("valid", func(t *testing.T) {
		e.users.profiles[userID] = true
		expect(t, e.do(http.MethodGet, "/api/users/me", sign(t, secret, userID, sid, time.Now().Add(time.Minute)), ""), http.StatusOK, "")
	})
}

func TestSessionStoreDown(t *testing.T) {
	e := newEnv(t)
	_, tok := e.login(t, true)
	e.sessions.err = errors.New("redis down")
	expect(t, e.do(http.MethodGet, "/api/users/me", tok, ""), http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
}

func TestClientUserIDHeaderIgnored(t *testing.T) {
	e := newEnv(t)
	userID, tok := e.login(t, true)

	// Без токена заголовок не даёт доступа.
	expect(t, e.do(http.MethodGet, "/api/users/me", "", "", "X-User-ID", userID), http.StatusUnauthorized, "UNAUTHORIZED")

	w := e.do(http.MethodGet, "/api/users/me", tok, "", "X-User-ID", uuid.NewString(), "X-Request-ID", "req-1")
	expect(t, w, http.StatusOK, "")
	if got := e.seen.get(grpcx.MDUserID); got != userID {
		t.Fatalf("service saw x-user-id %q, want %q", got, userID)
	}
	if got := e.seen.get(grpcx.MDRequestID); got != "req-1" {
		t.Fatalf("service saw x-request-id %q", got)
	}
	if w.Header().Get("X-Request-ID") != "req-1" || w.Header().Get("traceparent") == "" {
		t.Fatalf("trace headers missing: %v", w.Header())
	}
	if e.seen.get(grpcx.MDTraceParent) == "" {
		t.Fatal("traceparent not propagated")
	}
}

func TestProfileRequired(t *testing.T) {
	e := newEnv(t)
	_, tok := e.login(t, false)

	// Без профиля доступны управление аккаунтом и сам профиль.
	expect(t, e.do(http.MethodGet, "/api/auth/sessions", tok, ""), http.StatusOK, "")
	expect(t, e.do(http.MethodGet, "/api/users/me", tok, ""), http.StatusNotFound, "PROFILE_NOT_FOUND")

	blocked := []struct{ method, path, body string }{
		{http.MethodGet, "/api/conversations", ""},
		{http.MethodPost, "/api/messages", `{"content":"hi"}`},
		{http.MethodGet, "/api/users?q=a", ""},
		{http.MethodGet, "/api/users/" + uuid.NewString(), ""},
		{http.MethodPatch, "/api/users/me", `{}`},
		{http.MethodPost, "/api/conversations/" + uuid.NewString() + "/join", ""},
		{http.MethodGet, "/api/invites/abc", ""},
	}
	for _, b := range blocked {
		expect(t, e.do(b.method, b.path, tok, b.body), http.StatusForbidden, "PROFILE_REQUIRED")
	}

	w := e.do(http.MethodPost, "/api/users", tok, `{"username":"me","display_name":"Me"}`)
	expect(t, w, http.StatusCreated, "")
	if m := decode(t, w); m["success"] != true {
		t.Fatalf("body %v", m)
	}

	calls := e.users.existsCalls.Load()
	expect(t, e.do(http.MethodGet, "/api/conversations", tok, ""), http.StatusOK, "")
	expect(t, e.do(http.MethodGet, "/api/users?q=a", tok, ""), http.StatusOK, "")
	if got := e.users.existsCalls.Load(); got != calls {
		t.Fatalf("UserExists called %d more times after profile creation", got-calls)
	}
}

func TestProfileCachedAfterLookup(t *testing.T) {
	e := newEnv(t)
	_, tok := e.login(t, true)
	for range 3 {
		expect(t, e.do(http.MethodGet, "/api/conversations", tok, ""), http.StatusOK, "")
	}
	if got := e.users.existsCalls.Load(); got != 1 {
		t.Fatalf("UserExists calls = %d, want 1", got)
	}
}

func TestErrorMapping(t *testing.T) {
	e := newEnv(t)
	_, tok := e.login(t, true)
	body := `{"recipient_id":"` + uuid.NewString() + `","content":"secret text"}`

	cases := []struct {
		name   string
		err    error
		status int
		reason string
		msg    string
	}{
		{"domain", grpcx.Error(codes.PermissionDenied, "BLOCKED_BY_ME", "Вы заблокировали пользователя"), 403, "BLOCKED_BY_ME", "Вы заблокировали пользователя"},
		{"conflict", grpcx.Error(codes.FailedPrecondition, "LAST_ADMIN", "Нельзя"), 409, "LAST_ADMIN", "Нельзя"},
		{"plain not found", status.Error(codes.NotFound, ""), 404, "", "Не найдено"},
		{"internal hides details", status.Error(codes.Internal, "pq: relation does not exist"), 500, "", "Внутренняя ошибка сервера"},
		{"unavailable", status.Error(codes.Unavailable, ""), 503, "SERVICE_UNAVAILABLE", "Сервис временно недоступен"},
		{"timeout", status.Error(codes.DeadlineExceeded, ""), 504, "TIMEOUT", "Сервис не ответил вовремя"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e.convs.sendErr = c.err
			w := e.do(http.MethodPost, "/api/messages", tok, body)
			expect(t, w, c.status, c.reason)
			m := decode(t, w)
			if m["error"] != c.msg {
				t.Fatalf("error = %v, want %q", m["error"], c.msg)
			}
			if _, ok := m["code"]; ok {
				t.Fatal("unexpected code field")
			}
			if c.reason == "" {
				if _, ok := m["reason"]; ok {
					t.Fatalf("unexpected reason: %v", m)
				}
			}
		})
	}

	e.convs.sendErr = nil
	w := e.do(http.MethodPost, "/api/messages", tok, body)
	expect(t, w, http.StatusCreated, "")
	if m := decode(t, w); m["content"] != "secret text" || m["reply_to_message_id"] != nil {
		t.Fatalf("message body %v", m)
	}
}

func TestBadJSONAndBodyLimit(t *testing.T) {
	e := newEnv(t)
	_, tok := e.login(t, true)

	expect(t, e.do(http.MethodPost, "/api/messages", tok, `{"content":`), http.StatusBadRequest, "INVALID_REQUEST")

	big := `{"content":"` + strings.Repeat("a", 1<<20) + `"}`
	expect(t, e.do(http.MethodPost, "/api/messages", tok, big), http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE")
}

func TestNotFoundRoute(t *testing.T) {
	e := newEnv(t)
	expect(t, e.do(http.MethodGet, "/api/nope", "", ""), http.StatusNotFound, "NOT_FOUND")
}

func refreshCookie(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == "refresh_token" {
			return c
		}
	}
	t.Fatalf("no refresh_token cookie in %v", w.Header())
	return nil
}

func TestCookies(t *testing.T) {
	e := newEnv(t)

	t.Run("otp verify sets cookie", func(t *testing.T) {
		w := e.do(http.MethodPost, "/api/auth/otp/verify", "", `{"challenge_id":"x","code":"123456"}`)
		expect(t, w, http.StatusOK, "")
		m := decode(t, w)
		if m["access_token"] != "access" || m["token_type"] != "Bearer" || m["refresh_token"] != nil {
			t.Fatalf("body %v", m)
		}
		c := refreshCookie(t, w)
		if c.Value != "refresh-new" || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Secure || c.Path != "/" {
			t.Fatalf("cookie %+v", c)
		}
	})

	t.Run("secure only via trusted proxy", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/api/auth/otp/verify", strings.NewReader(`{}`))
		r.Header.Set("X-Forwarded-Proto", "https")
		w := httptest.NewRecorder()
		e.router.ServeHTTP(w, r)
		if refreshCookie(t, w).Secure {
			t.Fatal("Secure set for untrusted X-Forwarded-Proto")
		}

		r = httptest.NewRequest(http.MethodPost, "/api/auth/otp/verify", strings.NewReader(`{}`))
		r.RemoteAddr = "10.1.2.3:5555"
		r.Header.Set("X-Forwarded-Proto", "https")
		w = httptest.NewRecorder()
		e.router.ServeHTTP(w, r)
		if !refreshCookie(t, w).Secure {
			t.Fatal("Secure not set behind trusted proxy")
		}
	})

	t.Run("refresh without cookie", func(t *testing.T) {
		expect(t, e.do(http.MethodPost, "/api/auth/refresh", "", ""), http.StatusUnauthorized, "UNAUTHORIZED")
	})

	t.Run("refresh rotates cookie", func(t *testing.T) {
		w := e.do(http.MethodPost, "/api/auth/refresh", "", "", "Cookie", "refresh_token=refresh-old")
		expect(t, w, http.StatusOK, "")
		if refreshCookie(t, w).Value != "refresh-new" {
			t.Fatal("cookie not rotated")
		}
	})

	t.Run("invalid refresh clears cookie", func(t *testing.T) {
		w := e.do(http.MethodPost, "/api/auth/refresh", "", "", "Cookie", "refresh_token=stale")
		expect(t, w, http.StatusUnauthorized, "INVALID_TOKEN")
		if c := refreshCookie(t, w); c.MaxAge >= 0 || c.Value != "" {
			t.Fatalf("cookie not cleared: %+v", c)
		}
	})

	t.Run("logout clears cookie", func(t *testing.T) {
		w := e.do(http.MethodPost, "/api/auth/logout", "", "", "Cookie", "refresh_token=refresh-old")
		expect(t, w, http.StatusOK, "")
		if c := refreshCookie(t, w); c.MaxAge >= 0 {
			t.Fatalf("cookie not cleared: %+v", c)
		}
	})
}

func TestHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fail := false
	router, err := api.NewRouter(api.Deps{
		Verifier: auth.NewVerifier(secret, &fakeSessions{}),
		Health: []api.HealthCheck{{Name: "dep", Check: func(context.Context) error {
			if fail {
				return errors.New("down")
			}
			return nil
		}}},
		Log: zap.NewNop(),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		fail   bool
		status int
	}{{false, 200}, {true, 503}} {
		fail = tc.fail
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
		if w.Code != tc.status {
			t.Fatalf("health fail=%v: %d", tc.fail, w.Code)
		}
	}
}

func TestInvalidTrustedProxy(t *testing.T) {
	if _, err := api.NewRouter(api.Deps{TrustedProxies: []string{"nginx"}, Log: zap.NewNop()}); err == nil {
		t.Fatal("expected error")
	}
}
