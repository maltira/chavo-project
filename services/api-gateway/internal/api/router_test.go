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
	avatarshapev1 "github.com/maltira/chavo-project-backend/proto/gen/go/avatarshape/v1"
	conversationv1 "github.com/maltira/chavo-project-backend/proto/gen/go/conversation/v1"
	statusv1 "github.com/maltira/chavo-project-backend/proto/gen/go/status/v1"
	userv1 "github.com/maltira/chavo-project-backend/proto/gen/go/user/v1"
	"github.com/maltira/chavo-project-backend/proto/grpcx"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/api"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/auth"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/profile"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/ratelimit"
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
	avatars     map[string]string
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
	u.mu.Lock()
	avatar := u.avatars[id]
	u.mu.Unlock()
	p := &userv1.Profile{UserId: id, Username: "me"}
	if avatar != "" {
		p.AvatarUrl = &avatar
	}
	return &userv1.GetMeResponse{Profile: p}, nil
}

// UpdateMe запоминает только аватарку: проверку пути делает user-service, здесь важен порядок вызовов Gateway
func (u *fakeUsers) UpdateMe(ctx context.Context, req *userv1.UpdateMeRequest) (*userv1.UpdateMeResponse, error) {
	if req.AvatarUrl != nil {
		u.mu.Lock()
		u.avatars[grpcx.IncomingUserID(ctx)] = req.GetAvatarUrl()
		u.mu.Unlock()
	}
	return &userv1.UpdateMeResponse{}, nil
}

// SetStatus проверяет только иконку (полная проверка — в user-service) и возвращает статус как есть.
func (u *fakeUsers) SetStatus(_ context.Context, req *userv1.SetStatusRequest) (*userv1.SetStatusResponse, error) {
	if req.GetIcon() != "shine" {
		return nil, grpcx.Error(codes.InvalidArgument, "STATUS_ICON_INVALID", "Неизвестная иконка статуса")
	}
	return &userv1.SetStatusResponse{Status: &statusv1.Status{
		Icon: req.GetIcon(), Text: req.GetText(), Background: req.GetBackground(), Foreground: req.GetForeground(),
	}}, nil
}

func (u *fakeUsers) ClearStatus(context.Context, *userv1.ClearStatusRequest) (*userv1.ClearStatusResponse, error) {
	return &userv1.ClearStatusResponse{}, nil
}

// SetAvatarShape проверяет только ключ (полная проверка - в user-service) и возвращает фигуру как есть
func (u *fakeUsers) SetAvatarShape(_ context.Context, req *userv1.SetAvatarShapeRequest) (*userv1.SetAvatarShapeResponse, error) {
	if req.GetShape() != "bun" {
		return nil, grpcx.Error(codes.InvalidArgument, "AVATAR_SHAPE_INVALID", "Неизвестная фигура")
	}
	return &userv1.SetAvatarShapeResponse{AvatarShape: &avatarshapev1.AvatarShape{Shape: req.GetShape(), Color: req.GetColor()}}, nil
}

func (u *fakeUsers) ClearAvatarShape(context.Context, *userv1.ClearAvatarShapeRequest) (*userv1.ClearAvatarShapeResponse, error) {
	return &userv1.ClearAvatarShapeResponse{}, nil
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
	mu      sync.Mutex
	avatars map[string]string // группа -> путь аватарки
	admins  map[string]string // группа -> её админ
	// lastArchived - фильтр архива в последнем ListConversations
	lastArchived bool
}

func (f *fakeConversations) GetConversation(_ context.Context, req *conversationv1.GetConversationRequest) (*conversationv1.GetConversationResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.admins[req.GetConversationId()]; !ok {
		return nil, grpcx.Error(codes.NotFound, "", "Запись не найдена")
	}
	c := &conversationv1.Conversation{Id: req.GetConversationId(), ConversationType: "group"}
	if a := f.avatars[req.GetConversationId()]; a != "" {
		c.AvatarUrl = &a
	}
	return &conversationv1.GetConversationResponse{Conversation: &conversationv1.ConversationSummary{Conversation: c}}, nil
}

func (f *fakeConversations) UpdateGroup(ctx context.Context, req *conversationv1.UpdateGroupRequest) (*conversationv1.UpdateGroupResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.admins[req.GetConversationId()] != grpcx.IncomingUserID(ctx) {
		return nil, grpcx.Error(codes.PermissionDenied, "", "Недостаточно прав")
	}
	if req.AvatarUrl != nil {
		f.avatars[req.GetConversationId()] = req.GetAvatarUrl()
	}
	return &conversationv1.UpdateGroupResponse{Conversation: &conversationv1.Conversation{Id: req.GetConversationId()}}, nil
}

func (f *fakeConversations) ListConversations(_ context.Context, req *conversationv1.ListConversationsRequest) (*conversationv1.ListConversationsResponse, error) {
	f.mu.Lock()
	f.lastArchived = req.GetArchived()
	f.mu.Unlock()
	return &conversationv1.ListConversationsResponse{Limit: 30}, nil
}

// SetChatMute отвечает сроком как временем: так тест видит, что ушло в сервис
func (f *fakeConversations) SetChatMute(_ context.Context, req *conversationv1.SetChatMuteRequest) (*conversationv1.SetChatMuteResponse, error) {
	cs := &conversationv1.ChatSettings{ConversationId: req.GetConversationId()}
	switch {
	case req.Duration == nil:
	case req.GetDuration() == "8h":
		cs.MutedUntil = timestamppb.New(time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC))
	default:
		return nil, grpcx.Error(codes.InvalidArgument, "MUTE_DURATION_INVALID", "Неизвестный срок отключения звука")
	}
	return &conversationv1.SetChatMuteResponse{Settings: cs}, nil
}

func (*fakeConversations) SetChatArchived(_ context.Context, req *conversationv1.SetChatArchivedRequest) (*conversationv1.SetChatArchivedResponse, error) {
	cs := &conversationv1.ChatSettings{ConversationId: req.GetConversationId()}
	if req.GetArchived() {
		cs.ArchivedAt = timestamppb.Now()
	}
	return &conversationv1.SetChatArchivedResponse{Settings: cs}, nil
}

func (*fakeConversations) SetChatPinned(_ context.Context, req *conversationv1.SetChatPinnedRequest) (*conversationv1.SetChatPinnedResponse, error) {
	if req.GetPinned() {
		return nil, grpcx.Error(codes.FailedPrecondition, "PIN_LIMIT", "Можно закрепить не больше 5 чатов")
	}
	return &conversationv1.SetChatPinnedResponse{Settings: &conversationv1.ChatSettings{ConversationId: req.GetConversationId()}}, nil
}

func (f *fakeConversations) SendMessage(_ context.Context, req *conversationv1.SendMessageRequest) (*conversationv1.SendMessageResponse, error) {
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
	media    *memStore
}

type memCounter struct {
	mu     sync.Mutex
	counts map[string]int64
}

func (m *memCounter) Incr(_ context.Context, key string, _ time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counts[key]++
	return m.counts[key], nil
}

var generousLimits = api.Limits{
	API:      ratelimit.Rule{Name: "api", Limit: 1000, Window: time.Minute},
	Messages: ratelimit.Rule{Name: "messages", Limit: 1000, Window: time.Minute},
}

func newEnv(t *testing.T) *env { return newEnvWithLimits(t, generousLimits) }

func newEnvWithLimits(t *testing.T, limits api.Limits) *env {
	t.Helper()
	gin.SetMode(gin.TestMode)

	e := &env{
		sessions: &fakeSessions{owners: map[string]string{}},
		cache:    &fakeCache{set: map[string]bool{}},
		seen:     &seenMD{},
		convs:    &fakeConversations{avatars: map[string]string{}, admins: map[string]string{}},
		media:    &memStore{objects: map[string][]byte{}},
	}
	e.users = &fakeUsers{seen: e.seen, profiles: map[string]bool{}, avatars: map[string]string{}}

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
		Limiter:        ratelimit.New(&memCounter{counts: map[string]int64{}}),
		Limits:         limits,
		Media:          e.media,
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

func TestRateLimitPerUser(t *testing.T) {
	e := newEnvWithLimits(t, api.Limits{
		API:      ratelimit.Rule{Name: "api", Limit: 4, Window: time.Minute},
		Messages: ratelimit.Rule{Name: "messages", Limit: 2, Window: 10 * time.Second},
	})
	_, alice := e.login(t, true)
	_, bob := e.login(t, true)
	send := func(token string) *httptest.ResponseRecorder {
		return e.do(http.MethodPost, "/api/messages", token, `{"conversation_id":"`+uuid.NewString()+`","content":"hi"}`)
	}

	// Отправка сообщений ограничена строже общего лимита.
	for i := range 2 {
		if w := send(alice); w.Code == http.StatusTooManyRequests {
			t.Fatalf("message %d limited", i)
		}
	}
	w := send(alice)
	expect(t, w, http.StatusTooManyRequests, "RATE_LIMITED")
	if ra := w.Header().Get("Retry-After"); ra == "" || ra == "0" {
		t.Fatalf("Retry-After %q", ra)
	}

	// Общий лимит засчитал все три отправки; четвёртый запрос — последний разрешённый.
	if w := e.do(http.MethodGet, "/api/conversations", alice, ""); w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	expect(t, e.do(http.MethodGet, "/api/conversations", alice, ""), http.StatusTooManyRequests, "RATE_LIMITED")

	// Лимит у каждого пользователя свой.
	if w := e.do(http.MethodGet, "/api/conversations", bob, ""); w.Code != http.StatusOK {
		t.Fatalf("other user limited: %d", w.Code)
	}
	// Публичные маршруты лимитом по пользователю не покрыты (их режет nginx по IP).
	for range 6 {
		if w := e.do(http.MethodPost, "/api/auth/logout", "", ""); w.Code == http.StatusTooManyRequests {
			t.Fatal("public route limited")
		}
	}
}

func TestStatusEndpoints(t *testing.T) {
	e := newEnv(t)

	// каталог публичный и кешируемый: иконки и сроки статусов, фигуры
	w := e.do(http.MethodGet, "/api/customization", "", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Header().Get("Cache-Control"), "max-age") {
		t.Fatalf("catalog: %d %v", w.Code, w.Header())
	}
	cat := decode(t, w)
	st, _ := cat["status"].(map[string]any)
	icons, _ := st["icons"].([]any)
	durations, _ := st["durations"].([]any)
	shapes, _ := cat["avatar_shapes"].([]any)
	mutes, _ := cat["mute_durations"].([]any)
	if len(icons) != 6 || icons[0] != "moon" || len(durations) != 5 || len(shapes) != 27 || shapes[0] != "circle" ||
		len(mutes) != 4 || mutes[3] != "forever" {
		t.Fatalf("catalog content: %v", cat)
	}
	if w := e.do(http.MethodGet, "/api/statuses", "", ""); w.Code != http.StatusNotFound {
		t.Fatalf("old catalog route: %d", w.Code)
	}

	// Без профиля поставить статус нельзя.
	_, noProfile := e.login(t, false)
	expect(t, e.do(http.MethodPost, "/api/users/me/status", noProfile, `{"icon":"shine","text":"x","background":"#000000","foreground":"#FFFFFF","duration":"1h"}`),
		http.StatusForbidden, "PROFILE_REQUIRED")

	_, tok := e.login(t, true)
	w = e.do(http.MethodPost, "/api/users/me/status", tok, `{"icon":"shine","text":"Отдыхаю","background":"#34D399","foreground":"#064E3B","duration":"4h"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("set: %d %s", w.Code, w.Body.String())
	}
	st = decode(t, w)
	if st["icon"] != "shine" || st["text"] != "Отдыхаю" || st["background"] != "#34D399" || st["foreground"] != "#064E3B" {
		t.Fatalf("status: %v", st)
	}
	if _, ok := st["type"]; ok {
		t.Fatalf("status must not have a type: %v", st)
	}
	expect(t, e.do(http.MethodPost, "/api/users/me/status", tok, `{"icon":"party","text":"x","background":"#000000","foreground":"#FFFFFF","duration":"1h"}`),
		http.StatusBadRequest, "STATUS_ICON_INVALID")
	if w := e.do(http.MethodDelete, "/api/users/me/status", tok, ""); w.Code != http.StatusOK {
		t.Fatalf("clear: %d %s", w.Code, w.Body.String())
	}
	// PUT не поддерживается: статус только ставится заново или удаляется.
	if w := e.do(http.MethodPut, "/api/users/me/status", tok, `{}`); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("PUT: %d", w.Code)
	}
}

func TestAvatarShapeEndpoints(t *testing.T) {
	e := newEnv(t)

	_, noProfile := e.login(t, false)
	expect(t, e.do(http.MethodPost, "/api/users/me/avatar-shape", noProfile, `{"shape":"bun","color":"#000000"}`),
		http.StatusForbidden, "PROFILE_REQUIRED")

	_, tok := e.login(t, true)
	w := e.do(http.MethodPost, "/api/users/me/avatar-shape", tok, `{"shape":"bun","color":"#34D399"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("set: %d %s", w.Code, w.Body.String())
	}
	if sh := decode(t, w); sh["shape"] != "bun" || sh["color"] != "#34D399" {
		t.Fatalf("shape: %v", sh)
	}
	expect(t, e.do(http.MethodPost, "/api/users/me/avatar-shape", tok, `{"shape":"hexagon","color":"#000000"}`),
		http.StatusBadRequest, "AVATAR_SHAPE_INVALID")
	if w := e.do(http.MethodDelete, "/api/users/me/avatar-shape", tok, ""); w.Code != http.StatusOK {
		t.Fatalf("clear: %d %s", w.Code, w.Body.String())
	}
	// PUT не поддерживается: фигура только ставится заново или удаляется
	if w := e.do(http.MethodPut, "/api/users/me/avatar-shape", tok, `{}`); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("PUT: %d", w.Code)
	}
}

func TestChatSettingsEndpoints(t *testing.T) {
	e := newEnv(t)
	_, tok := e.login(t, true)
	id := uuid.NewString()

	w := e.do(http.MethodPost, "/api/conversations/"+id+"/mute", tok, `{"duration":"8h"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("mute: %d %s", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if body["conversation_id"] != id || body["muted_until"] != "2026-01-01T08:00:00Z" {
		t.Fatalf("mute body: %v", body)
	}
	expect(t, e.do(http.MethodPost, "/api/conversations/"+id+"/mute", tok, `{"duration":"3h"}`),
		http.StatusBadRequest, "MUTE_DURATION_INVALID")
	if body = decode(t, e.do(http.MethodDelete, "/api/conversations/"+id+"/mute", tok, "")); body["muted_until"] != nil {
		t.Fatalf("unmute body: %v", body)
	}

	if body = decode(t, e.do(http.MethodPost, "/api/conversations/"+id+"/archive", tok, "")); body["archived_at"] == nil {
		t.Fatalf("archive body: %v", body)
	}
	if body = decode(t, e.do(http.MethodDelete, "/api/conversations/"+id+"/archive", tok, "")); body["archived_at"] != nil {
		t.Fatalf("unarchive body: %v", body)
	}
	expect(t, e.do(http.MethodPost, "/api/conversations/"+id+"/pin", tok, ""), http.StatusConflict, "PIN_LIMIT")
	if w := e.do(http.MethodDelete, "/api/conversations/"+id+"/pin", tok, ""); w.Code != http.StatusOK {
		t.Fatalf("unpin: %d", w.Code)
	}

	// архив - отдельный список
	e.do(http.MethodGet, "/api/conversations?archived=true", tok, "")
	if !e.convs.lastArchived {
		t.Fatal("archived filter not passed")
	}
	e.do(http.MethodGet, "/api/conversations", tok, "")
	if e.convs.lastArchived {
		t.Fatal("main list asked for archive")
	}
}
