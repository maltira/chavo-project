package hub

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

type fakeRegistry struct {
	mu   sync.Mutex
	sets map[string]map[string]bool
	err  error
}

func newRegistry() *fakeRegistry { return &fakeRegistry{sets: map[string]map[string]bool{}} }

func (r *fakeRegistry) Add(_ context.Context, userID, connID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	if r.sets[userID] == nil {
		r.sets[userID] = map[string]bool{}
	}
	r.sets[userID][connID] = true
	return nil
}

func (r *fakeRegistry) Remove(_ context.Context, userID, connID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sets[userID], connID)
	return nil
}

func (r *fakeRegistry) count(userID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.sets[userID])
}

func register(t *testing.T, h *Hub, id, user, sid string) *Conn {
	t.Helper()
	c := NewConn(id, user, sid, 4)
	if _, err := h.Register(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	return c
}

func closed(c *Conn) bool {
	select {
	case <-c.Done():
		return true
	default:
		return false
	}
}

func TestRegisterFirstLast(t *testing.T) {
	reg := newRegistry()
	h := New(reg, zap.NewNop())

	a := NewConn("s1:a", "u1", "s1", 4)
	first, err := h.Register(context.Background(), a)
	if err != nil || !first {
		t.Fatalf("first = %v, err = %v", first, err)
	}
	b := NewConn("s1:b", "u1", "s1", 4)
	if first, _ := h.Register(context.Background(), b); first {
		t.Fatal("second connection reported as first")
	}
	if reg.count("u1") != 2 || !h.Online("u1") {
		t.Fatal("connections not registered")
	}

	if h.Unregister(a) {
		t.Fatal("last reported while another tab is open")
	}
	if !h.Unregister(b) {
		t.Fatal("last not reported")
	}
	if h.Unregister(b) {
		t.Fatal("repeated Unregister must be a no-op")
	}
	if reg.count("u1") != 0 || h.Online("u1") || h.Len() != 0 {
		t.Fatal("connections not removed")
	}
}

func TestRegisterFailsWithoutRegistry(t *testing.T) {
	reg := newRegistry()
	reg.err = errors.New("redis down")
	h := New(reg, zap.NewNop())
	if _, err := h.Register(context.Background(), NewConn("s:a", "u", "s", 1)); err == nil {
		t.Fatal("expected error")
	}
	if h.Len() != 0 {
		t.Fatal("connection registered despite registry error")
	}
}

func TestRouting(t *testing.T) {
	h := New(newRegistry(), zap.NewNop())
	tab1 := register(t, h, "s1:a", "u1", "s1")
	tab2 := register(t, h, "s1:b", "u1", "s1")
	phone := register(t, h, "s2:a", "u1", "s2")
	other := register(t, h, "s3:a", "u2", "s3")

	h.SendToUsers([]string{"u1", "nobody"}, []byte("m1"))
	for _, c := range []*Conn{tab1, tab2, phone} {
		if got := string(<-c.Send()); got != "m1" {
			t.Fatalf("%s got %q", c.ID, got)
		}
	}
	if len(other.Send()) != 0 {
		t.Fatal("message leaked to another user")
	}

	h.SendToSID("s2", []byte("m2"))
	if len(tab1.Send()) != 0 || string(<-phone.Send()) != "m2" {
		t.Fatal("SendToSID routed wrong")
	}

	h.CloseSID("s1", []byte("bye"), CloseSessionRevoked, "session revoked")
	if !closed(tab1) || !closed(tab2) || closed(phone) || closed(other) {
		t.Fatal("CloseSID closed wrong connections")
	}
	if code, _, final := tab1.CloseInfo(); code != CloseSessionRevoked || string(final) != "bye" {
		t.Fatalf("close info %d %q", code, final)
	}

	h.CloseUser("u1", nil, CloseGoingAway, "")
	if !closed(phone) || closed(other) {
		t.Fatal("CloseUser closed wrong connections")
	}
}

func TestSlowConsumerClosed(t *testing.T) {
	c := NewConn("s:a", "u", "s", 1)
	if !c.Enqueue([]byte("1")) {
		t.Fatal("first enqueue failed")
	}
	if c.Enqueue([]byte("2")) {
		t.Fatal("overflow accepted")
	}
	if code, _, _ := c.CloseInfo(); !closed(c) || code != CloseTryAgainLater {
		t.Fatalf("slow consumer not closed: %d", code)
	}
	if c.Enqueue([]byte("3")) {
		t.Fatal("enqueue after close accepted")
	}
}

func TestShutdown(t *testing.T) {
	reg := newRegistry()
	h := New(reg, zap.NewNop())
	c := register(t, h, "s:a", "u", "s")

	// Горутины соединения снимают его с учёта, увидев Done.
	go func() {
		<-c.Done()
		h.Unregister(c)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := h.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := c.CloseInfo(); code != CloseGoingAway {
		t.Fatalf("close code %d", code)
	}
	if reg.count("u") != 0 {
		t.Fatal("registry not cleaned")
	}
	if _, err := h.Register(context.Background(), NewConn("s:b", "u", "s", 1)); !errors.Is(err, ErrClosed) {
		t.Fatalf("register after shutdown: %v", err)
	}
}

func TestWatchers(t *testing.T) {
	h := New(newRegistry(), zap.NewNop())
	a := register(t, h, "s1:a", "u1", "s1")
	b := register(t, h, "s2:a", "u2", "s2")

	h.WatchDirect(a, []string{"peer", "u1"}) // себя не наблюдают
	if err := h.Subscribe(a, []string{"peer", "x"}, 2); err != nil {
		t.Fatal(err)
	}
	if err := h.Subscribe(a, []string{"y"}, 2); !errors.Is(err, ErrWatchLimit) {
		t.Fatalf("limit: %v", err)
	}
	h.SendToWatchers("u1", []byte("self"))
	if len(a.Send()) != 0 {
		t.Fatal("connection watches its own user")
	}

	// Отписка от собеседника по direct его не снимает, от остальных — снимает.
	h.Unsubscribe(a, []string{"peer", "x"})
	h.SendToWatchers("peer", []byte("p"))
	h.SendToWatchers("x", []byte("x"))
	if len(a.Send()) != 1 || string(<-a.Send()) != "p" {
		t.Fatal("unsubscribe routed wrong")
	}

	h.WatchPeers("u1", "u2")
	h.SendToWatchers("u2", []byte("to-a"))
	h.SendToWatchers("u1", []byte("to-b"))
	if string(<-a.Send()) != "to-a" || string(<-b.Send()) != "to-b" {
		t.Fatal("WatchPeers routed wrong")
	}

	h.Unregister(a)
	h.Unregister(b)
	h.mu.RLock()
	n := len(h.watchers)
	h.mu.RUnlock()
	if n != 0 {
		t.Fatalf("watchers left after unregister: %d", n)
	}
	// Подписка закрытого соединения ничего не добавляет.
	h.WatchDirect(a, []string{"peer"})
	if err := h.Subscribe(a, []string{"x"}, 2); err != nil || len(h.watchers) != 0 {
		t.Fatal("closed connection subscribed")
	}
}
