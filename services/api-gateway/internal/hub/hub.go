// Package hub — реестр WebSocket-соединений Gateway: индексы по sid и user_id, адресная доставка, учёт в Redis.
// Ввод-вывод сокета живёт в пакете ws; здесь соединение — очередь исходящих сообщений и сигнал закрытия.
package hub

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Коды закрытия WebSocket (RFC 6455 и диапазон приложения 4000–4999).
const (
	CloseGoingAway      = 1001
	ClosePolicy         = 1008
	CloseInternal       = 1011
	CloseTryAgainLater  = 1013
	CloseSessionRevoked = 4001
	CloseUnauthorized   = 4401
	CloseProfileRequired = 4403
)

// ErrClosed — Hub остановлен и новых соединений не принимает.
var ErrClosed = errors.New("hub closed")

// Registry отражает живые соединения в Redis (ws:user:<user_id> = set connection_id) для user-service.
type Registry interface {
	Add(ctx context.Context, userID, connID string) error
	Remove(ctx context.Context, userID, connID string) error
}

// Conn — одно WS-соединение (вкладка). ID имеет вид <sid>:<суффикс>.
type Conn struct {
	ID     string
	UserID string
	SID    string

	send   chan []byte
	done   chan struct{}
	once   sync.Once
	code   int
	reason string
	final  []byte
}

func NewConn(id, userID, sid string, buffer int) *Conn {
	return &Conn{ID: id, UserID: userID, SID: sid, send: make(chan []byte, buffer), done: make(chan struct{})}
}

// Send — исходящие сообщения для writer-горутины.
func (c *Conn) Send() <-chan []byte { return c.send }

// Done закрывается, когда соединение нужно завершить.
func (c *Conn) Done() <-chan struct{} { return c.done }

// Close просит завершить соединение; повторные вызовы ничего не меняют.
func (c *Conn) Close(code int, reason string) { c.CloseWith(nil, code, reason) }

// CloseWith — то же, но перед кадром закрытия клиенту уходит final.
func (c *Conn) CloseWith(final []byte, code int, reason string) {
	c.once.Do(func() {
		c.code, c.reason, c.final = code, reason, final
		close(c.done)
	})
}

// CloseInfo — код, причина и последнее сообщение; читать только после Done.
func (c *Conn) CloseInfo() (code int, reason string, final []byte) {
	return c.code, c.reason, c.final
}

// Enqueue кладёт сообщение в очередь без блокировки. Переполнение очереди — медленный клиент: соединение закрывается,
// после переподключения клиент синхронизируется через REST.
func (c *Conn) Enqueue(msg []byte) bool {
	select {
	case <-c.done:
		return false
	default:
	}
	select {
	case c.send <- msg:
		return true
	default:
		c.Close(CloseTryAgainLater, "slow consumer")
		return false
	}
}

type Hub struct {
	registry Registry
	log      *zap.Logger

	mu     sync.RWMutex
	conns  map[string]*Conn
	bySID  map[string]map[string]*Conn
	byUser map[string]map[string]*Conn
	closed bool
	wg     sync.WaitGroup
}

func New(registry Registry, log *zap.Logger) *Hub {
	return &Hub{
		registry: registry,
		log:      log,
		conns:    map[string]*Conn{},
		bySID:    map[string]map[string]*Conn{},
		byUser:   map[string]map[string]*Conn{},
	}
}

// Register добавляет соединение; first = это первое соединение пользователя.
// Без записи в Redis соединение не принимается: иначе user-service считал бы пользователя офлайн.
func (h *Hub) Register(ctx context.Context, c *Conn) (first bool, err error) {
	h.mu.RLock()
	closed := h.closed
	h.mu.RUnlock()
	if closed {
		return false, ErrClosed
	}
	if err := h.registry.Add(ctx, c.UserID, c.ID); err != nil {
		return false, err
	}

	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		h.removeFromRegistry(c)
		return false, ErrClosed
	}
	h.conns[c.ID] = c
	add(h.bySID, c.SID, c)
	first = add(h.byUser, c.UserID, c)
	h.wg.Add(1)
	h.mu.Unlock()
	return first, nil
}

// Unregister убирает соединение (вызывается ровно один раз после завершения его горутин); last = соединений
// пользователя больше нет.
func (h *Hub) Unregister(c *Conn) (last bool) {
	h.mu.Lock()
	if _, ok := h.conns[c.ID]; !ok {
		h.mu.Unlock()
		return false
	}
	delete(h.conns, c.ID)
	remove(h.bySID, c.SID, c.ID)
	last = remove(h.byUser, c.UserID, c.ID)
	h.mu.Unlock()

	h.removeFromRegistry(c)
	h.wg.Done()
	return last
}

func (h *Hub) removeFromRegistry(c *Conn) {
	// Контекст запроса к этому моменту уже отменён.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := h.registry.Remove(ctx, c.UserID, c.ID); err != nil {
		h.log.Error("ws registry remove failed", zap.String("connection_id", c.ID), zap.Error(err))
	}
}

// SendToUsers доставляет сообщение во все соединения перечисленных пользователей.
func (h *Hub) SendToUsers(userIDs []string, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, id := range userIDs {
		for _, c := range h.byUser[id] {
			c.Enqueue(msg)
		}
	}
}

// SendToSID доставляет сообщение во все вкладки одной сессии.
func (h *Hub) SendToSID(sid string, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.bySID[sid] {
		c.Enqueue(msg)
	}
}

// CloseSID закрывает все вкладки сессии, отправив перед этим final.
func (h *Hub) CloseSID(sid string, final []byte, code int, reason string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.bySID[sid] {
		c.CloseWith(final, code, reason)
	}
}

// CloseUser закрывает все соединения пользователя.
func (h *Hub) CloseUser(userID string, final []byte, code int, reason string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.byUser[userID] {
		c.CloseWith(final, code, reason)
	}
}

// Online — есть ли у пользователя соединения на этом Gateway.
func (h *Hub) Online(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.byUser[userID]) > 0
}

func (h *Hub) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}

// Shutdown перестаёт принимать соединения, закрывает существующие и ждёт их снятия с учёта.
func (h *Hub) Shutdown(ctx context.Context) error {
	h.mu.Lock()
	h.closed = true
	for _, c := range h.conns {
		c.Close(CloseGoingAway, "server shutdown")
	}
	h.mu.Unlock()

	done := make(chan struct{})
	go func() {
		h.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// add возвращает true, если для key это первое соединение.
func add(index map[string]map[string]*Conn, key string, c *Conn) bool {
	set, ok := index[key]
	if !ok {
		set = map[string]*Conn{}
		index[key] = set
	}
	set[c.ID] = c
	return !ok
}

// remove возвращает true, если для key соединений не осталось.
func remove(index map[string]map[string]*Conn, key, id string) bool {
	set, ok := index[key]
	if !ok {
		return false
	}
	delete(set, id)
	if len(set) == 0 {
		delete(index, key)
		return true
	}
	return false
}
