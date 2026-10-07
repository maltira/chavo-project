//go:build e2e

// Package e2e — сквозные тесты chavo на поднятом стеке compose: настоящий клиент ходит через nginx
// (HTTP и WebSocket), письма перехватывает Mailpit. Запуск: make e2e.
package e2e

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestMain(m *testing.M) {
	if err := preflight(); err != nil {
		fmt.Fprintln(os.Stderr, "e2e: "+err.Error())
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// preflight не даёт тесту слать настоящие письма и стартовать на неготовом стеке.
func preflight() error {
	if cfg.pgUser == "" || cfg.authDB == "" || cfg.userDB == "" || cfg.convDB == "" {
		return fmt.Errorf("нет POSTGRES_USER / *_DB_NAME — запускайте через make e2e (переменные берутся из .env)")
	}
	out, err := exec.Command("docker", "exec", "chavo-auth", "printenv", "SMTP_HOST").Output()
	if err != nil {
		return fmt.Errorf("auth-service не запущен (make up): %v", err)
	}
	if host := strings.TrimSpace(string(out)); host != "mailpit" {
		return fmt.Errorf("auth-service отправляет почту через %q, а не Mailpit: тест разослал бы настоящие письма. "+
			"Укажите в .env SMTP_HOST=mailpit, SMTP_PORT=1025, SMTP_USER=noreply@chavo.local и пересоздайте auth-service", host)
	}
	for _, u := range []string{cfg.base + "/nginx-health", cfg.mailpit + "/api/v1/info"} {
		r, err := http.Get(u)
		if err != nil {
			return fmt.Errorf("%s недоступен: %v", u, err)
		}
		r.Body.Close()
		if r.StatusCode != http.StatusOK {
			return fmt.Errorf("%s: %d", u, r.StatusCode)
		}
	}
	return nil
}

func check(t *testing.T, ok bool, format string, args ...any) {
	t.Helper()
	if !ok {
		t.Errorf(format, args...)
	}
}

// step запускает шаг; если от него зависят следующие, провал останавливает сценарий.
func step(t *testing.T, name string, critical bool, f func(t *testing.T)) {
	t.Helper()
	if !t.Run(name, f) && critical {
		t.FailNow()
	}
}

func TestScenario(t *testing.T) {
	started := time.Now()
	w := &world{run: randomHex(4), mail: &mailbox{used: map[string]bool{}}}
	t.Cleanup(func() { w.cleanup(t) })
	secret := "e2e-secret-" + w.run + "-текст сообщения"
	w.secrets = append(w.secrets, secret, password)

	var (
		a, b, c          *user
		phone            *device
		tab1, tab2, tabP *wsConn
		bw, cw           *wsConn
		sidA             string
		msgID, convID    string
	)

	step(t, "public", false, func(t *testing.T) {
		anon := newDevice()
		r := anon.do(t, "GET", "/api/users/me", nil)
		check(t, r.status == 401 && r.body["reason"] == "UNAUTHORIZED", "no token: %d %s", r.status, r.raw)
		r = anon.do(t, "GET", "/", nil)
		check(t, r.status == 200, "frontend placeholder: %d", r.status)
		if conn, _, err := websocket.DefaultDialer.Dial(cfg.wsURL, http.Header{"Origin": {"http://evil.example"}}); err == nil {
			conn.Close()
			t.Error("WS from foreign origin accepted")
		}
	})

	// Регистрация и вход через настоящие письма: ссылка подтверждения и OTP — из Mailpit.
	step(t, "register and login via email", true, func(t *testing.T) {
		a, b, c = w.newUser(t, "a"), w.newUser(t, "b"), w.newUser(t, "c")
	})

	step(t, "profile required", true, func(t *testing.T) {
		r := a.dev.do(t, "GET", "/api/conversations", nil)
		check(t, r.status == 403 && r.body["reason"] == "PROFILE_REQUIRED", "conversations: %d %s", r.status, r.raw)
		r = a.dev.do(t, "GET", "/api/users/me", nil)
		check(t, r.status == 404, "users/me: %d %s", r.status, r.raw)
		if ws, err := dial("a-noprofile", a.dev); err == nil {
			f := ws.wait("error", 5*time.Second)
			code := ws.closeCode(5 * time.Second)
			check(t, f != nil && f["reason"] == "PROFILE_REQUIRED" && code == 4403, "WS: %v %d", f, code)
		} else {
			t.Error(err)
		}

		for _, u := range []*user{a, b, c} {
			r := u.dev.do(t, "POST", "/api/users", map[string]string{"username": u.username, "display_name": "E2E " + u.username})
			if r.status != 201 {
				t.Fatalf("create profile: %d %s", r.status, r.raw)
			}
			r = u.dev.do(t, "GET", "/api/users/me", nil)
			u.id, _ = r.body["user_id"].(string)
		}
		r = a.dev.do(t, "GET", "/api/conversations", nil)
		check(t, r.status == 200, "after profile: %d %s", r.status, r.raw)
		r = a.dev.do(t, "GET", "/api/users/me", nil, "X-User-ID", b.id)
		check(t, r.body["user_id"] == a.id, "client X-User-ID must be ignored, got %v", r.body["user_id"])
	})

	step(t, "websocket tabs and devices", true, func(t *testing.T) {
		phone = newDevice()
		w.login(t, phone, a.email)
		sidA = sidOf(a.dev.access)
		check(t, sidOf(phone.access) != sidA, "second login must get a new sid")

		tab1, tab2 = w.mustDial(t, "a-tab1", a.dev), w.mustDial(t, "a-tab2", a.dev)
		tabP = w.mustDial(t, "a-phone", phone)
		bw, cw = w.mustDial(t, "b", b.dev), w.mustDial(t, "c", c.dev)
		check(t, strings.HasPrefix(tab1.id, sidA+":") && strings.HasPrefix(tab2.id, sidA+":") && tab1.id != tab2.id,
			"connection ids: %s %s", tab1.id, tab2.id)
		check(t, redis(t, "SCARD", "ws:user:"+a.id) == "3", "ws:user registry of A: %s", redis(t, "SCARD", "ws:user:"+a.id))
		check(t, tab1.ping(), "ping → pong")
		cw.send(map[string]string{"type": "nope"})
		f := cw.wait("error", 3*time.Second)
		check(t, f != nil && f["reason"] == "UNKNOWN_TYPE", "unknown type: %v", f)

		r := b.dev.do(t, "GET", "/api/users/"+a.id, nil)
		check(t, r.body["online"] == true, "REST online: %s", r.raw)

		cw.send(map[string]any{"type": "presence.subscribe", "user_ids": []string{a.id, b.id}})
		f = cw.wait("presence.snapshot", 5*time.Second)
		check(t, f != nil && usersIn(f)[a.id]["online"] == true, "subscribe snapshot: %v", f)
	})

	step(t, "messages", true, func(t *testing.T) {
		r := a.dev.do(t, "POST", "/api/messages", map[string]string{"recipient_id": b.id, "content": secret})
		if r.status != 201 {
			t.Fatalf("send: %d %s", r.status, r.raw)
		}
		msgID, convID = r.body["id"].(string), r.body["conversation_id"].(string)

		for _, ws := range []*wsConn{bw, tab1, tab2, tabP} {
			f := ws.waitWhere("message.created", 10*time.Second, func(f frame) bool { return f.data()["message_id"] == msgID })
			check(t, f != nil && f.data()["content"] == secret && f.data()["member_ids"] == nil &&
				f.data()["content_enc"] == nil && f["event_id"] != nil, "message.created → %s: %v", ws.name, f)
		}
		// Новый direct: собеседники видят присутствие друг друга без переподключения.
		f := bw.waitWhere("presence.snapshot", 10*time.Second, func(f frame) bool { return usersIn(f)[a.id] != nil })
		check(t, f != nil && usersIn(f)[a.id]["online"] == true, "B gets A presence: %v", f)
		f = tab1.waitWhere("presence.snapshot", 10*time.Second, func(f frame) bool { return usersIn(f)[b.id] != nil })
		check(t, f != nil, "A gets B presence")
		check(t, cw.wait("message.created", 2*time.Second) == nil, "outsider C got the message")

		r = b.dev.do(t, "POST", "/api/conversations/"+convID+"/read", map[string]string{"message_id": msgID})
		check(t, r.status == 200, "mark read: %d %s", r.status, r.raw)
		f = tab1.waitWhere("message.read", 10*time.Second, func(f frame) bool { return f.data()["reader_id"] == b.id })
		check(t, f != nil && f.data()["author_ids"] == nil, "message.read → author: %v", f)
		check(t, bw.waitWhere("message.read", 10*time.Second, func(f frame) bool { return f.data()["reader_id"] == b.id }) != nil,
			"message.read → reader's own tabs")

		r = a.dev.do(t, "PATCH", "/api/messages/"+msgID, map[string]string{"content": secret + " (ред.)"})
		check(t, r.status == 200, "edit: %d %s", r.status, r.raw)
		f = bw.waitWhere("message.updated", 10*time.Second, func(f frame) bool { return f.data()["message_id"] == msgID })
		check(t, f != nil && f.data()["content"] == secret+" (ред.)", "message.updated: %v", f)

		r = a.dev.do(t, "GET", "/api/messages?conversation_id="+convID, nil)
		items, _ := r.body["items"].([]any)
		check(t, r.status == 200 && len(items) == 1, "history: %d %s", r.status, r.raw)
	})

	step(t, "blocking", false, func(t *testing.T) {
		r := b.dev.do(t, "POST", "/api/users/"+a.id+"/block", nil)
		check(t, r.status == 200 || r.status == 201, "block: %d %s", r.status, r.raw)
		blocked := func(f frame) bool { return f.data()["blocked_id"] == a.id }
		check(t, tab1.waitWhere("user.blocked", 10*time.Second, blocked) != nil, "user.blocked → blocked side")
		check(t, bw.waitWhere("user.blocked", 10*time.Second, blocked) != nil, "user.blocked → blocker side")
		r = a.dev.do(t, "POST", "/api/messages", map[string]string{"conversation_id": convID, "content": "x"})
		check(t, r.status == 403 && r.body["reason"] != nil, "send while blocked: %d %s", r.status, r.raw)
		r = b.dev.do(t, "DELETE", "/api/users/"+a.id+"/block", nil)
		check(t, r.status == 200, "unblock: %d %s", r.status, r.raw)
		check(t, tab1.wait("user.unblocked", 10*time.Second) != nil, "user.unblocked → A")

		r = a.dev.do(t, "DELETE", "/api/messages/"+msgID, nil)
		check(t, r.status == 200, "delete: %d %s", r.status, r.raw)
		check(t, bw.waitWhere("message.deleted", 10*time.Second, func(f frame) bool { return f.data()["message_id"] == msgID }) != nil,
			"message.deleted → B")
	})

	step(t, "presence offline and last_seen_at", false, func(t *testing.T) {
		r := a.dev.do(t, "GET", "/api/users/"+b.id, nil)
		seenBefore, _ := r.body["last_seen_at"].(string)
		tab1.drain()
		closedAt := time.Now()
		bw.closeNormally()

		check(t, tab1.waitWhere("user.offline", 7*time.Second, about(b.id)) == nil, "offline came before the 10 s delay")
		f := tab1.waitWhere("user.offline", 8*time.Second, about(b.id))
		if f == nil {
			t.Fatalf("no user.offline for direct peer after %v", time.Since(closedAt))
		}
		check(t, time.Since(closedAt) >= 9*time.Second, "offline after %v", time.Since(closedAt))
		check(t, f.data()["last_seen_at"] != nil, "offline without last_seen_at: %v", f)
		check(t, cw.waitWhere("user.offline", 3*time.Second, about(b.id)) != nil, "explicit subscriber C got no offline")

		// user-service обновляет last_seen_at по presence-events.
		var seenAfter string
		for i := 0; i < 20; i++ {
			r = a.dev.do(t, "GET", "/api/users/"+b.id, nil)
			seenAfter, _ = r.body["last_seen_at"].(string)
			if seenAfter != seenBefore && r.body["online"] == false {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		check(t, seenAfter != "" && seenAfter != seenBefore && r.body["online"] == false,
			"last_seen_at not updated: before=%s after=%s online=%v", seenBefore, seenAfter, r.body["online"])

		// Возврат до истечения задержки: перехода нет.
		bw = w.mustDial(t, "b2", b.dev)
		check(t, tab1.waitWhere("user.online", 5*time.Second, about(b.id)) != nil, "A sees B online again")
		bw.closeNormally()
		time.Sleep(3 * time.Second)
		bw = w.mustDial(t, "b3", b.dev)
		check(t, tab1.waitWhere("user.offline", 12*time.Second, about(b.id)) == nil, "reconnect within delay flapped offline")
	})

	step(t, "sessions", false, func(t *testing.T) {
		r := a.dev.do(t, "GET", "/api/auth/sessions", nil)
		before := map[string]bool{}
		for _, s := range r.list {
			before[s.(map[string]any)["id"].(string)] = true
		}
		r = a.dev.do(t, "POST", "/api/auth/refresh", nil)
		check(t, r.status == 200, "refresh: %d %s", r.status, r.raw)
		a.dev.access, _ = r.body["access_token"].(string)
		w.secrets = append(w.secrets, a.dev.access, a.dev.refreshCookie())
		check(t, sidOf(a.dev.access) == sidA, "refresh changed sid")
		r = a.dev.do(t, "GET", "/api/auth/sessions", nil)
		same := len(r.list) == len(before)
		for _, s := range r.list {
			same = same && before[s.(map[string]any)["id"].(string)]
		}
		check(t, same, "session list changed by refresh: %v vs %s", before, r.raw)

		r = a.dev.do(t, "DELETE", "/api/auth/sessions/"+sidOf(phone.access), nil)
		check(t, r.status == 200, "terminate phone: %d %s", r.status, r.raw)
		f := tabP.wait("session.revoked", 10*time.Second)
		code := tabP.closeCode(5 * time.Second)
		check(t, f != nil && f.data()["reason"] == "remote_logout" && code == 4001, "phone WS: %v %d", f, code)
		check(t, tab1.ping() && tab2.ping(), "tabs of the other sid dropped")
		r = phone.do(t, "GET", "/api/conversations", nil)
		check(t, r.status == 401, "revoked access token: %d", r.status)
	})

	step(t, "gateway rate limits", false, func(t *testing.T) {
		first429 := 0
		var r resp
		for i := 1; i <= 32; i++ {
			r = c.dev.do(t, "POST", "/api/messages", map[string]string{"recipient_id": b.id, "content": "flood"})
			if r.status == 429 {
				first429 = i
				break
			}
		}
		check(t, first429 == 31 && r.body["reason"] == "RATE_LIMITED" && r.header.Get("Retry-After") != "",
			"messages limit 30/10s: first 429 at #%d %s", first429, r.raw)
		for i := 0; i < 25; i++ {
			cw.send(map[string]string{"type": "ping"})
		}
		check(t, cw.waitWhere("error", 5*time.Second, func(f frame) bool { return f["reason"] == "RATE_LIMITED" }) != nil,
			"WS incoming limit")
	})

	step(t, "logout closes all tabs of the session", false, func(t *testing.T) {
		r := a.dev.do(t, "POST", "/api/auth/logout", nil)
		check(t, r.status == 200, "logout: %d %s", r.status, r.raw)
		for _, ws := range []*wsConn{tab1, tab2} {
			f := ws.wait("session.revoked", 10*time.Second)
			code := ws.closeCode(5 * time.Second)
			check(t, f != nil && f.data()["reason"] == "logout" && code == 4001, "%s: %v %d", ws.name, f, code)
		}
		check(t, redis(t, "SCARD", "ws:user:"+a.id) == "0", "A still in ws registry")
	})

	step(t, "gateway restart", false, func(t *testing.T) {
		r := b.dev.do(t, "GET", "/api/users/"+c.id, nil)
		cSeenBefore, _ := r.body["last_seen_at"].(string)
		sh(t, "docker", "restart", "chavo-gateway")
		check(t, cw.closeCode(15*time.Second) == 1001, "WS not closed with 1001 on shutdown")
		for i := 0; i < 60; i++ {
			if strings.Contains(sh(t, "docker", "inspect", "-f", "{{.State.Health.Status}}", "chavo-gateway"), "healthy") {
				break
			}
			time.Sleep(time.Second)
		}
		keys := redis(t, "--scan", "--pattern", "ws:user:*")
		check(t, keys == "", "ws:user:* after restart: %s", keys)
		var cSeenAfter string
		for i := 0; i < 20; i++ {
			r = b.dev.do(t, "GET", "/api/users/"+c.id, nil)
			cSeenAfter, _ = r.body["last_seen_at"].(string)
			if r.status == 200 && cSeenAfter != cSeenBefore {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		check(t, cSeenAfter != cSeenBefore, "shutdown did not flush user.offline: %s → %s", cSeenBefore, cSeenAfter)
		check(t, w.mustDial(t, "c-after-restart", c.dev).ping(), "WS after restart")
	})

	// Лимит nginx по IP — последним: после него /api/auth/ с этого адреса минуту отвечает 429.
	step(t, "nginx auth rate limit", false, func(t *testing.T) {
		noAuthRetry = true
		defer func() { noAuthRetry = false }()
		anon := newDevice()
		var r resp
		for i := 0; i < 25; i++ {
			r = anon.do(t, "POST", "/api/auth/login", map[string]string{"email": "nobody@example.com", "password": "x"})
			if r.status == 429 {
				break
			}
		}
		check(t, r.status == 429 && r.body["reason"] == "RATE_LIMITED", "nginx limit: %d %s", r.status, r.raw)
	})

	// Безопасность: токены, пароль и текст сообщений не должны попадать в логи.
	step(t, "no secrets in logs", false, func(t *testing.T) {
		since := started.Add(-time.Second).Format(time.RFC3339)
		for _, name := range []string{"chavo-nginx", "chavo-gateway", "chavo-auth", "chavo-user", "chavo-conversation"} {
			out, _ := exec.Command("docker", "logs", "--since", since, name).CombinedOutput()
			for i, s := range w.secrets {
				if s != "" && strings.Contains(string(out), s) {
					t.Errorf("secret #%d leaked into %s logs", i, name)
				}
			}
		}
	})
}
