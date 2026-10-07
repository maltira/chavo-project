//go:build e2e

package e2e

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ── окружение ────────────────────────────────────────

type config struct {
	base, wsURL, origin, mailpit string
	pgUser                       string
	authDB, userDB, convDB       string
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func loadConfig() config {
	port := env("NGINX_PORT", "8080")
	return config{
		base:    "http://localhost:" + port,
		wsURL:   "ws://localhost:" + port + "/ws",
		origin:  env("FRONTEND_URL", "http://localhost:3000"),
		mailpit: "http://localhost:" + env("MAILPIT_UI_PORT", "8025"),
		pgUser:  os.Getenv("POSTGRES_USER"),
		authDB:  os.Getenv("AUTH_DB_NAME"),
		userDB:  os.Getenv("USER_DB_NAME"),
		convDB:  os.Getenv("CONVERSATION_DB_NAME"),
	}
}

var cfg = loadConfig()

func sh(t testing.TB, args ...string) string {
	t.Helper()
	out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func redis(t testing.TB, args ...string) string {
	t.Helper()
	return sh(t, append([]string{"docker", "exec", "chavo-redis", "redis-cli"}, args...)...)
}

func psql(t testing.TB, db, sql string) string {
	t.Helper()
	return sh(t, "docker", "exec", "chavo-postgres", "psql", "-U", cfg.pgUser, "-d", db, "-At", "-v", "ON_ERROR_STOP=1", "-c", sql)
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ── Mailpit ──────────────────────────────────────────

type mailbox struct {
	used map[string]bool
}

type mailSummary struct {
	ID      string `json:"ID"`
	Subject string `json:"Subject"`
}

// wait ждёт новое письмо на адрес с темой subject (пустая — любая) и возвращает его текст.
func (m *mailbox) wait(t testing.TB, to, subject string) string {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var res struct {
			Messages []mailSummary `json:"messages"`
		}
		getJSON(t, cfg.mailpit+"/api/v1/search?query="+url.QueryEscape(`to:"`+to+`"`), &res)
		for _, msg := range res.Messages {
			if m.used[msg.ID] || (subject != "" && msg.Subject != subject) {
				continue
			}
			m.used[msg.ID] = true
			var full struct {
				Text string `json:"Text"`
				HTML string `json:"HTML"`
			}
			getJSON(t, cfg.mailpit+"/api/v1/message/"+msg.ID, &full)
			return full.Text + "\n" + full.HTML
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("no email to %s (subject %q)", to, subject)
	return ""
}

func getJSON(t testing.TB, u string, v any) {
	t.Helper()
	r, err := http.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		t.Fatalf("%s: %v", u, err)
	}
}

var (
	verifyTokenRe = regexp.MustCompile(`register/verify\?token=([A-Za-z0-9_-]+)`)
	otpRe         = regexp.MustCompile(`\b(\d{6})\b`)
)

// ── HTTP-клиент «устройства» ─────────────────────────

type device struct {
	http   *http.Client
	access string
}

func newDevice() *device {
	jar, _ := cookiejar.New(nil)
	return &device{http: &http.Client{Jar: jar, Timeout: 15 * time.Second}}
}

type resp struct {
	status int
	body   map[string]any
	list   []any
	raw    string
	header http.Header
}

// noAuthRetry отключает повтор при лимите nginx — для проверки самого лимита.
var noAuthRetry bool

func (d *device) do(t testing.TB, method, path string, body any, hdr ...string) resp {
	t.Helper()
	for attempt := 0; ; attempt++ {
		var rd io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, cfg.base+path, rd)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if d.access != "" {
			req.Header.Set("Authorization", "Bearer "+d.access)
		}
		for i := 0; i+1 < len(hdr); i += 2 {
			req.Header.Set(hdr[i], hdr[i+1])
		}
		r, err := d.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(r.Body)
		r.Body.Close()
		res := resp{status: r.StatusCode, raw: string(raw), header: r.Header}
		_ = json.Unmarshal(raw, &res.body)
		_ = json.Unmarshal(raw, &res.list)
		// Лимит nginx на /api/auth/ (10/мин) задевает и сам сценарий: ждём и повторяем.
		if r.StatusCode == http.StatusTooManyRequests && strings.HasPrefix(path, "/api/auth/") && !noAuthRetry && attempt < 20 {
			time.Sleep(7 * time.Second)
			continue
		}
		return res
	}
}

func (d *device) refreshCookie() string {
	u, _ := url.Parse(cfg.base)
	for _, c := range d.http.Jar.Cookies(u) {
		if c.Name == "refresh_token" {
			return c.Value
		}
	}
	return ""
}

func sidOf(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	b, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims map[string]any
	_ = json.Unmarshal(b, &claims)
	s, _ := claims["sid"].(string)
	return s
}

// ── пользователи ─────────────────────────────────────

const password = "E2e-Passw0rd!"

type user struct {
	email, id, username string
	dev                 *device
}

type world struct {
	run  string // префикс тестовых данных этого прогона
	mail *mailbox
	// Секреты для проверки логов: не должны встречаться в выводе контейнеров.
	secrets []string
	// conns закрываются в конце сценария, а не шага: соединения переживают шаг, в котором открыты.
	conns []*wsConn
}

func (w *world) newUser(t testing.TB, tag string) *user {
	t.Helper()
	u := &user{
		email:    fmt.Sprintf("e2e-%s-%s@example.com", w.run, tag),
		username: fmt.Sprintf("e2e_%s_%s", w.run, tag),
		dev:      newDevice(),
	}
	r := u.dev.do(t, "POST", "/api/auth/register", map[string]string{"email": u.email, "password": password})
	if r.status != http.StatusCreated {
		t.Fatalf("register: %d %s", r.status, r.raw)
	}
	m := verifyTokenRe.FindStringSubmatch(w.mail.wait(t, u.email, "Подтвердите ваш аккаунт на Chavo"))
	if m == nil {
		t.Fatal("verification link not found in email")
	}
	if r := u.dev.do(t, "GET", "/api/auth/register/verify?token="+m[1], nil); r.status != http.StatusOK {
		t.Fatalf("verify email: %d %s", r.status, r.raw)
	}
	w.login(t, u.dev, u.email)
	return u
}

// login проходит вход с OTP из письма.
func (w *world) login(t testing.TB, d *device, email string) {
	t.Helper()
	r := d.do(t, "POST", "/api/auth/login", map[string]string{"email": email, "password": password})
	if r.status != http.StatusOK {
		t.Fatalf("login %s: %d %s", email, r.status, r.raw)
	}
	ch, _ := r.body["challenge_id"].(string)
	m := otpRe.FindStringSubmatch(w.mail.wait(t, email, ""))
	if m == nil {
		t.Fatal("OTP code not found in email")
	}
	r = d.do(t, "POST", "/api/auth/otp/verify", map[string]string{"challenge_id": ch, "code": m[1]})
	if r.status != http.StatusOK {
		t.Fatalf("otp verify %s: %d %s", email, r.status, r.raw)
	}
	d.access, _ = r.body["access_token"].(string)
	w.secrets = append(w.secrets, d.access, d.refreshCookie())
}

// cleanup закрывает соединения и удаляет всё, что оставил прогон: пользователей, профили, диалоги,
// ключи Redis, письма.
func (w *world) cleanup(t testing.TB) {
	for _, c := range w.conns {
		_ = c.conn.Close()
	}
	pattern := "e2e-" + w.run + "-%@example.com"
	ids := strings.Fields(psql(t, cfg.authDB, fmt.Sprintf(
		`SELECT id FROM users WHERE email LIKE '%s'`, pattern)))
	if len(ids) == 0 {
		return
	}
	quoted := "'" + strings.Join(ids, "','") + "'"
	sids := strings.Fields(psql(t, cfg.authDB, fmt.Sprintf(
		`SELECT id FROM refresh_tokens WHERE user_id::text IN (%s)`, quoted)))

	// Диалоги, где все участники — тестовые; их события в outbox связаны через event_key.
	psql(t, cfg.convDB, fmt.Sprintf(`BEGIN;
		CREATE TEMP TABLE e2e_conv AS
			SELECT DISTINCT conversation_id AS id FROM conversation_members WHERE user_id::text IN (%[1]s)
			EXCEPT SELECT conversation_id FROM conversation_members WHERE user_id::text NOT IN (%[1]s);
		DELETE FROM outbox_events WHERE event_key IN (SELECT id::text FROM e2e_conv);
		DELETE FROM conversations WHERE id IN (SELECT id FROM e2e_conv);
		COMMIT;`, quoted))
	psql(t, cfg.userDB, fmt.Sprintf(`BEGIN;
		DELETE FROM user_blocks WHERE user_id::text IN (%[1]s) OR blocked_user_id::text IN (%[1]s);
		DELETE FROM user_settings WHERE user_id::text IN (%[1]s);
		DELETE FROM profiles WHERE user_id::text IN (%[1]s);
		COMMIT;`, quoted))
	psql(t, cfg.authDB, fmt.Sprintf(`DELETE FROM users WHERE id::text IN (%s)`, quoted))

	keys := []string{}
	for _, id := range ids {
		keys = append(keys, "profile:"+id, "ws:user:"+id)
		keys = append(keys, strings.Fields(redis(t, "--scan", "--pattern", "rl:*:"+id+":*"))...)
	}
	for _, sid := range sids {
		keys = append(keys, "ws:session:"+sid)
	}
	redis(t, append([]string{"DEL"}, keys...)...)

	req, _ := http.NewRequest(http.MethodDelete,
		cfg.mailpit+"/api/v1/search?query="+url.QueryEscape("to:e2e-"+w.run), nil)
	if r, err := http.DefaultClient.Do(req); err == nil {
		r.Body.Close()
	}
}

// ── WebSocket ────────────────────────────────────────

type frame map[string]any

func (f frame) typ() string { s, _ := f["type"].(string); return s }
func (f frame) data() map[string]any {
	d, _ := f["data"].(map[string]any)
	return d
}

type wsConn struct {
	name   string
	conn   *websocket.Conn
	frames chan frame
	closed chan int
	id     string
	// backlog — кадры, пропущенные при ожидании другого типа: они ещё могут понадобиться.
	backlog []frame
}

func dial(name string, d *device) (*wsConn, error) {
	hdr := http.Header{"Origin": {cfg.origin}}
	if c := d.refreshCookie(); c != "" {
		hdr.Set("Cookie", "refresh_token="+c)
	}
	conn, _, err := websocket.DefaultDialer.Dial(cfg.wsURL, hdr)
	if err != nil {
		return nil, err
	}
	w := &wsConn{name: name, conn: conn, frames: make(chan frame, 256), closed: make(chan int, 1)}
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				code := -1
				if ce, ok := err.(*websocket.CloseError); ok {
					code = ce.Code
				}
				w.closed <- code
				close(w.frames)
				return
			}
			var f frame
			_ = json.Unmarshal(data, &f)
			w.frames <- f
		}
	}()
	return w, nil
}

func (wd *world) mustDial(t testing.TB, name string, d *device) *wsConn {
	t.Helper()
	w, err := dial(name, d)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	wd.conns = append(wd.conns, w)
	f := w.wait("ready", 5*time.Second)
	if f == nil {
		t.Fatalf("%s: no ready frame", name)
	}
	w.id, _ = f["connection_id"].(string)
	return w
}

func (w *wsConn) wait(typ string, timeout time.Duration) frame {
	return w.waitWhere(typ, timeout, func(frame) bool { return true })
}

// waitWhere ждёт кадр типа typ, удовлетворяющий cond; остальные кадры откладываются в backlog.
func (w *wsConn) waitWhere(typ string, timeout time.Duration, cond func(frame) bool) frame {
	for i, f := range w.backlog {
		if f.typ() == typ && cond(f) {
			w.backlog = append(w.backlog[:i:i], w.backlog[i+1:]...)
			return f
		}
	}
	deadline := time.After(timeout)
	for {
		select {
		case f, ok := <-w.frames:
			if !ok {
				return nil
			}
			if f.typ() == typ && cond(f) {
				return f
			}
			w.backlog = append(w.backlog, f)
		case <-deadline:
			return nil
		}
	}
}

// closeCode ждёт закрытия соединения и возвращает его код (0 — не закрылось).
func (w *wsConn) closeCode(timeout time.Duration) int {
	deadline := time.After(timeout)
	for {
		select {
		case _, ok := <-w.frames:
			if !ok {
				return <-w.closed
			}
		case <-deadline:
			return 0
		}
	}
}

func (w *wsConn) send(v any) {
	b, _ := json.Marshal(v)
	_ = w.conn.WriteMessage(websocket.TextMessage, b)
}

func (w *wsConn) closeNormally() {
	_ = w.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
}

func (w *wsConn) drain() {
	w.backlog = nil
	for {
		select {
		case _, ok := <-w.frames:
			if !ok {
				return
			}
		default:
			return
		}
	}
}

func (w *wsConn) ping() bool {
	w.send(map[string]string{"type": "ping"})
	return w.wait("pong", 3*time.Second) != nil
}

func usersIn(f frame) map[string]map[string]any {
	res := map[string]map[string]any{}
	list, _ := f.data()["users"].([]any)
	for _, raw := range list {
		e, _ := raw.(map[string]any)
		if id, ok := e["user_id"].(string); ok {
			res[id] = e
		}
	}
	return res
}

func about(id string) func(frame) bool {
	return func(f frame) bool { return f.data()["user_id"] == id }
}
