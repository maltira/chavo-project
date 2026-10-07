package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestParseRule(t *testing.T) {
	r, err := ParseRule("messages", " 30/10s ")
	if err != nil || r != (Rule{Name: "messages", Limit: 30, Window: 10 * time.Second}) {
		t.Fatalf("%+v %v", r, err)
	}
	for _, bad := range []string{"", "30", "0/1m", "-1/1m", "x/1m", "30/abc", "30/500ms", "30/1500ms"} {
		if _, err := ParseRule("x", bad); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
}

type counter struct {
	counts map[string]int64
	ttls   map[string]time.Duration
	err    error
}

func (c *counter) Incr(_ context.Context, key string, ttl time.Duration) (int64, error) {
	if c.err != nil {
		return 0, c.err
	}
	c.counts[key]++
	c.ttls[key] = ttl
	return c.counts[key], nil
}

func TestFixedWindow(t *testing.T) {
	c := &counter{counts: map[string]int64{}, ttls: map[string]time.Duration{}}
	l := New(c)
	now := time.Unix(1_000_000_005, 0) // 5 с от начала 10-секундного окна
	l.now = func() time.Time { return now }
	rule := Rule{Name: "messages", Limit: 2, Window: 10 * time.Second}
	ctx := context.Background()

	for i := range 2 {
		if ok, _, _ := l.Allow(ctx, rule, "u1"); !ok {
			t.Fatalf("request %d denied", i)
		}
	}
	ok, retry, err := l.Allow(ctx, rule, "u1")
	if ok || err != nil || retry != 5*time.Second {
		t.Fatalf("ok=%v retry=%v err=%v", ok, retry, err)
	}
	if ok, _, _ := l.Allow(ctx, rule, "u2"); !ok {
		t.Fatal("other key denied")
	}
	if c.ttls["rl:messages:u1:100000000"] != rule.Window {
		t.Fatalf("keys %v", c.ttls)
	}

	// Новое окно — новый счётчик.
	now = now.Add(5 * time.Second)
	if ok, _, _ := l.Allow(ctx, rule, "u1"); !ok {
		t.Fatal("next window denied")
	}
}

func TestFailOpen(t *testing.T) {
	l := New(&counter{err: errors.New("redis down")})
	if ok, _, err := l.Allow(context.Background(), Rule{Name: "x", Limit: 1, Window: time.Second}, "u"); !ok || err == nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}
