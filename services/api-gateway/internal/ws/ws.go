// Package ws — WebSocket-эндпоинт Gateway: handshake по refresh-cookie, чтение команд клиента и запись событий из Hub.
package ws

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	authv1 "github.com/maltira/chavo-project-backend/proto/gen/go/auth/v1"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/httpx"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/hub"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/presence"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/profile"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/reqctx"
)

const refreshCookie = "refresh_token"

const (
	ReasonForbiddenOrigin = "FORBIDDEN_ORIGIN"
	ReasonInvalidMessage  = "INVALID_MESSAGE"
	ReasonUnknownType     = "UNKNOWN_TYPE"
	ReasonPresenceLimit   = "PRESENCE_LIMIT"
)

type Options struct {
	// PingInterval — как часто сервер шлёт ping; PongWait — сколько ждать ответа (любого кадра) до разрыва.
	PingInterval time.Duration
	PongWait     time.Duration
	// WriteWait — таймаут записи одного кадра.
	WriteWait time.Duration
	// SendBuffer — очередь исходящих сообщений; переполнение закрывает соединение.
	SendBuffer int
	// MaxMessageBytes — лимит входящего сообщения клиента.
	MaxMessageBytes int64
}

func DefaultOptions() Options {
	return Options{
		PingInterval:    30 * time.Second,
		PongWait:        60 * time.Second,
		WriteWait:       10 * time.Second,
		SendBuffer:      256,
		MaxMessageBytes: 4 << 10,
	}
}

// Presence — учёт присутствия (пакет presence); nil — без presence.
type Presence interface {
	Connected(ctx context.Context, c *hub.Conn, first bool)
	Disconnected(c *hub.Conn, last bool)
	Subscribe(ctx context.Context, c *hub.Conn, userIDs []string) error
	Unsubscribe(c *hub.Conn, userIDs []string) error
}

type Deps struct {
	Auth     authv1.AuthServiceClient
	Profiles *profile.Gate
	Hub      *hub.Hub
	Presence Presence
	Origin   string
	Options  Options
	Log      *zap.Logger
}

type Handler struct {
	auth     authv1.AuthServiceClient
	profiles *profile.Gate
	hub      *hub.Hub
	presence Presence
	origin   string
	opts     Options
	log      *zap.Logger
	upgrader websocket.Upgrader
}

func New(d Deps) (*Handler, error) {
	origin, err := normalizeOrigin(d.Origin)
	if err != nil {
		return nil, fmt.Errorf("invalid frontend origin %q", d.Origin)
	}
	h := &Handler{auth: d.Auth, profiles: d.Profiles, hub: d.Hub, presence: d.Presence, origin: origin, opts: d.Options, log: d.Log}
	h.upgrader = websocket.Upgrader{
		HandshakeTimeout: 10 * time.Second,
		// Origin уже проверен в Handle.
		CheckOrigin: func(*http.Request) bool { return true },
		Error: func(w http.ResponseWriter, _ *http.Request, status int, _ error) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(httpx.ErrorBody{Error: "Некорректный запрос WebSocket", Reason: httpx.ReasonInvalidRequest})
		},
	}
	return h, nil
}

func normalizeOrigin(s string) (string, error) {
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", errors.New("invalid origin")
	}
	return strings.ToLower(u.Scheme + "://" + u.Host), nil
}

// ── сообщения протокола ──────────────────────────────

type readyMsg struct {
	Type         string `json:"type"`
	ConnectionID string `json:"connection_id"`
}

type errorMsg struct {
	Type   string `json:"type"`
	Reason string `json:"reason"`
	Error  string `json:"error"`
}

type clientMsg struct {
	Type    string   `json:"type"`
	UserIDs []string `json:"user_ids"`
}

func encode(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func errorFrame(reason, message string) []byte {
	return encode(errorMsg{Type: "error", Reason: reason, Error: message})
}

var pongFrame = []byte(`{"type":"pong"}`)

// ── handshake ────────────────────────────────────────

// Handle — GET /ws. Ошибки после upgrade приходят клиенту кадром {"type":"error"} и кодом закрытия:
// браузер не видит HTTP-статус неудачного handshake.
func (h *Handler) Handle(c *gin.Context) {
	if o, err := normalizeOrigin(c.GetHeader("Origin")); err != nil || o != h.origin {
		httpx.Abort(c, http.StatusForbidden, "Недопустимый источник запроса", ReasonForbiddenOrigin)
		return
	}

	// gin пишет заголовки лениво: статус нужен только для лога запроса.
	c.Status(http.StatusSwitchingProtocols)
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.Set(httpx.KeyErrorCode, httpx.ReasonInvalidRequest)
		return
	}

	ctx := c.Request.Context()
	userID, sid, reject := h.authenticate(ctx, c)
	if reject != nil {
		c.Set(httpx.KeyErrorCode, reject.reason)
		h.reject(conn, reject)
		return
	}

	info := reqctx.From(ctx)
	info.UserID, info.SessionID = userID, sid
	h.serve(ctx, conn, hub.NewConn(sid+":"+randomHex(8), userID, sid, h.opts.SendBuffer))
}

type rejection struct {
	reason, message string
	code            int
}

var (
	rejectUnauthorized = &rejection{httpx.ReasonUnauthorized, "Необходима авторизация", hub.CloseUnauthorized}
	rejectNoProfile    = &rejection{httpx.ReasonProfileRequired, "Необходимо заполнить профиль", hub.CloseProfileRequired}
	rejectUnavailable  = &rejection{httpx.ReasonUnavailable, "Сервис временно недоступен", hub.CloseTryAgainLater}
)

func (h *Handler) authenticate(ctx context.Context, c *gin.Context) (userID, sid string, rej *rejection) {
	token, err := c.Cookie(refreshCookie)
	if err != nil || token == "" {
		return "", "", rejectUnauthorized
	}
	resp, err := h.auth.ResolveSession(ctx, &authv1.ResolveSessionRequest{RefreshToken: token})
	if err != nil {
		if httpx.IsClientError(err) {
			return "", "", rejectUnauthorized
		}
		h.log.Warn("ws resolve session failed", zap.Error(err))
		return "", "", rejectUnavailable
	}

	ok, err := h.profiles.Exists(ctx, resp.GetUserId())
	if err != nil {
		h.log.Warn("ws profile check failed", zap.Error(err))
		return "", "", rejectUnavailable
	}
	if !ok {
		return "", "", rejectNoProfile
	}
	return resp.GetUserId(), resp.GetSessionId(), nil
}

func (h *Handler) reject(conn *websocket.Conn, r *rejection) {
	deadline := time.Now().Add(h.opts.WriteWait)
	_ = conn.SetWriteDeadline(deadline)
	_ = conn.WriteMessage(websocket.TextMessage, errorFrame(r.reason, r.message))
	_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(r.code, r.reason), deadline)
	_ = conn.Close()
}

// ── жизнь соединения ─────────────────────────────────

func (h *Handler) serve(ctx context.Context, conn *websocket.Conn, c *hub.Conn) {
	log := h.log.With(zap.String("connection_id", c.ID), zap.String("user_id", c.UserID), zap.String("session_id", c.SID),
		zap.String("request_id", reqctx.From(ctx).RequestID))

	first, err := h.hub.Register(ctx, c)
	if err != nil {
		log.Error("ws register failed", zap.Error(err))
		r := rejectUnavailable
		if errors.Is(err, hub.ErrClosed) {
			r = &rejection{httpx.ReasonUnavailable, "Сервер перезапускается", hub.CloseGoingAway}
		}
		h.reject(conn, r)
		return
	}
	started := time.Now()
	log.Info("ws connected")

	c.Enqueue(encode(readyMsg{Type: "ready", ConnectionID: c.ID}))
	if h.presence != nil {
		h.presence.Connected(ctx, c, first)
	}

	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		h.writeLoop(conn, c)
	}()
	h.readLoop(ctx, conn, c)
	c.Close(websocket.CloseNormalClosure, "")
	<-writerDone
	last := h.hub.Unregister(c)
	if h.presence != nil {
		h.presence.Disconnected(c, last)
	}

	code, reason, _ := c.CloseInfo()
	log.Info("ws disconnected", zap.Int("close_code", code), zap.String("close_reason", reason),
		zap.Duration("duration", time.Since(started)))
}

// readLoop читает команды клиента до ошибки чтения: закрытие вкладки, отсутствие pong, превышение лимита.
func (h *Handler) readLoop(ctx context.Context, conn *websocket.Conn, c *hub.Conn) {
	conn.SetReadLimit(h.opts.MaxMessageBytes)
	alive := func() { _ = conn.SetReadDeadline(time.Now().Add(h.opts.PongWait)) }
	alive()
	conn.SetPongHandler(func(string) error { alive(); return nil })

	for {
		kind, data, err := conn.ReadMessage()
		if err != nil {
			var ne net.Error
			switch {
			case errors.Is(err, websocket.ErrReadLimit):
				c.Close(websocket.CloseMessageTooBig, "message too big")
			case errors.As(err, &ne) && ne.Timeout():
				c.Close(hub.CloseGoingAway, "pong timeout")
			}
			return
		}
		alive()
		if kind != websocket.TextMessage {
			c.Enqueue(errorFrame(ReasonInvalidMessage, "Ожидается JSON-сообщение"))
			continue
		}
		h.dispatch(ctx, c, data)
	}
}

func (h *Handler) dispatch(ctx context.Context, c *hub.Conn, data []byte) {
	var msg clientMsg
	err := json.Unmarshal(data, &msg)
	if err != nil {
		c.Enqueue(errorFrame(ReasonInvalidMessage, "Некорректное сообщение"))
		return
	}
	switch msg.Type {
	case "ping":
		c.Enqueue(pongFrame)
	case "presence.subscribe", "presence.unsubscribe":
		if h.presence == nil {
			c.Enqueue(errorFrame(ReasonUnknownType, "Неизвестный тип сообщения"))
			return
		}
		if msg.Type == "presence.subscribe" {
			err = h.presence.Subscribe(ctx, c, msg.UserIDs)
		} else {
			err = h.presence.Unsubscribe(c, msg.UserIDs)
		}
		if err != nil {
			c.Enqueue(h.presenceError(c, err))
		}
	default:
		c.Enqueue(errorFrame(ReasonUnknownType, "Неизвестный тип сообщения"))
	}
}

func (h *Handler) presenceError(c *hub.Conn, err error) []byte {
	switch {
	case errors.Is(err, presence.ErrInvalidIDs):
		return errorFrame(ReasonInvalidMessage, "user_ids: ожидается непустой список UUID")
	case errors.Is(err, presence.ErrTooMany), errors.Is(err, hub.ErrWatchLimit):
		return errorFrame(ReasonPresenceLimit, "Слишком много подписок на статус")
	default:
		h.log.Warn("presence snapshot failed", zap.String("connection_id", c.ID), zap.Error(err))
		return errorFrame(httpx.ReasonUnavailable, "Сервис временно недоступен")
	}
}

// writeLoop — единственный писатель в сокет: события из очереди, ping по таймеру, финальный кадр и close.
func (h *Handler) writeLoop(conn *websocket.Conn, c *hub.Conn) {
	ticker := time.NewTicker(h.opts.PingInterval)
	defer func() {
		ticker.Stop()
		_ = conn.Close()
	}()

	for {
		select {
		case msg := <-c.Send():
			_ = conn.SetWriteDeadline(time.Now().Add(h.opts.WriteWait))
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				c.Close(hub.CloseInternal, "write failed")
				return
			}
		case <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(h.opts.WriteWait)); err != nil {
				c.Close(hub.CloseInternal, "ping failed")
				return
			}
		case <-c.Done():
			code, reason, final := c.CloseInfo()
			deadline := time.Now().Add(h.opts.WriteWait)
			if final != nil {
				_ = conn.SetWriteDeadline(deadline)
				_ = conn.WriteMessage(websocket.TextMessage, final)
			}
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), deadline)
			return
		}
	}
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
