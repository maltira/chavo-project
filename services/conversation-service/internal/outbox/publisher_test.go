package outbox_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/outbox"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/testutil"
)

type fakePub struct {
	mu     sync.Mutex
	got    []int
	topics []string
	keys   []string
	failAt int // номер вызова (с 1), на котором вернуть ошибку один раз; 0 — не падать
	calls  int
	delay  time.Duration
	block  bool // ждать отмены ctx
}

func (f *fakePub) Publish(ctx context.Context, topic, key string, event any) error {
	if f.block {
		<-ctx.Done()
		return ctx.Err()
	}
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.failAt != 0 && f.calls == f.failAt {
		return errors.New("kafka down")
	}
	var p struct {
		N int `json:"n"`
	}
	if err := json.Unmarshal(event.(json.RawMessage), &p); err != nil {
		return err
	}
	f.got = append(f.got, p.N)
	f.topics = append(f.topics, topic)
	f.keys = append(f.keys, key)
	return nil
}

func (f *fakePub) received() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int(nil), f.got...)
}

type fixture struct {
	pool *pgxpool.Pool
	db   *repository.DB
	repo repository.OutboxRepository
}

func setup(t *testing.T) *fixture {
	t.Helper()
	pool := testutil.Pool(t, "outbox_test")
	return &fixture{pool: pool, db: repository.NewDB(pool), repo: repository.NewOutboxRepository()}
}

func (f *fixture) seed(t *testing.T, from, to int) {
	t.Helper()
	for n := from; n <= to; n++ {
		topic := "message-events"
		if n%2 == 0 {
			topic = "conversation-events"
		}
		if err := f.repo.Insert(context.Background(), f.pool, topic, "conv-"+uuid.NewString()[:4], map[string]int{"n": n}); err != nil {
			t.Fatal(err)
		}
	}
}

func (f *fixture) publisher(pub outbox.EventPublisher, batch int) *outbox.Publisher {
	opts := outbox.DefaultOptions()
	opts.BatchSize = batch
	opts.PollInterval = 10 * time.Millisecond
	opts.PublishTimeout = time.Second
	return outbox.NewPublisher(f.db, f.repo, pub, testCipher(), zap.NewNop(), opts)
}

func (f *fixture) count(t *testing.T, where string) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM outbox_events WHERE `+where).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func seq(from, to int) []int {
	var s []int
	for i := from; i <= to; i++ {
		s = append(s, i)
	}
	return s
}

func equal(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestPublishesInOrderAndMarksPublished(t *testing.T) {
	f := setup(t)
	f.seed(t, 1, 5)
	pub := &fakePub{}

	n, err := f.publisher(pub, 100).RunOnce(context.Background())
	if err != nil || n != 5 {
		t.Fatalf("RunOnce = %d, %v", n, err)
	}
	if !equal(pub.received(), seq(1, 5)) {
		t.Fatalf("order = %v", pub.received())
	}
	if pub.topics[0] != "message-events" || pub.topics[1] != "conversation-events" || pub.keys[0] == "" {
		t.Fatalf("topic/key not forwarded: %v %v", pub.topics, pub.keys)
	}
	if f.count(t, "published_at IS NULL") != 0 {
		t.Fatal("all events must be marked published")
	}
	if n, _ = f.publisher(pub, 100).RunOnce(context.Background()); n != 0 {
		t.Fatalf("second run republished %d events", n)
	}
}

func TestBatchLimit(t *testing.T) {
	f := setup(t)
	f.seed(t, 1, 7)
	pub := &fakePub{}
	p := f.publisher(pub, 3)

	for _, want := range []int{3, 3, 1, 0} {
		if n, err := p.RunOnce(context.Background()); err != nil || n != want {
			t.Fatalf("RunOnce = %d, %v, want %d", n, err, want)
		}
	}
	if !equal(pub.received(), seq(1, 7)) {
		t.Fatalf("order = %v", pub.received())
	}
}

func TestFailureKeepsOrderAndRetries(t *testing.T) {
	f := setup(t)
	f.seed(t, 1, 5)
	pub := &fakePub{failAt: 3}
	p := f.publisher(pub, 100)

	n, err := p.RunOnce(context.Background())
	if err == nil || n != 2 {
		t.Fatalf("RunOnce = %d, %v; want 2 published and an error", n, err)
	}
	if f.count(t, "published_at IS NOT NULL") != 2 {
		t.Fatal("successful events must stay marked despite later failure")
	}
	if f.count(t, "attempts = 1 AND published_at IS NULL") != 1 || f.count(t, "attempts = 0 AND published_at IS NULL") != 2 {
		t.Fatal("only the failed event gets an attempt; later events must stay untouched")
	}

	if n, err = p.RunOnce(context.Background()); err != nil || n != 3 {
		t.Fatalf("retry = %d, %v", n, err)
	}
	if !equal(pub.received(), seq(1, 5)) {
		t.Fatalf("each event exactly once, in order; got %v", pub.received())
	}
}

func TestSingleActivePublisherPreservesOrder(t *testing.T) {
	f := setup(t)
	f.seed(t, 1, 120)
	pub := &fakePub{delay: time.Millisecond}

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := f.publisher(pub, 10)
			for f.count(t, "published_at IS NULL") > 0 {
				if _, err := p.RunOnce(context.Background()); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()

	if !equal(pub.received(), seq(1, 120)) {
		t.Fatalf("concurrent publishers broke order or duplicated events: %v", pub.received())
	}
}

func TestCancelledPublishRecordsFailure(t *testing.T) {
	f := setup(t)
	f.seed(t, 1, 2)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()

	n, err := f.publisher(&fakePub{block: true}, 10).RunOnce(ctx)
	if err == nil || n != 0 {
		t.Fatalf("RunOnce = %d, %v", n, err)
	}
	if f.count(t, "attempts = 1 AND published_at IS NULL") != 1 {
		t.Fatal("failure must be recorded even though ctx was cancelled")
	}
}

func TestRunPublishesAsynchronouslyAndStops(t *testing.T) {
	f := setup(t)
	pub := &fakePub{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { f.publisher(pub, 10).Run(ctx); close(done) }()

	f.seed(t, 1, 25)
	deadline := time.Now().Add(5 * time.Second)
	for len(pub.received()) < 25 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if !equal(pub.received(), seq(1, 25)) {
		t.Fatalf("got %v", pub.received())
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after cancel")
	}
}

func TestPurgeKeepsRecentAndUnpublished(t *testing.T) {
	f := setup(t)
	f.seed(t, 1, 3)
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, `UPDATE outbox_events SET published_at = now() - interval '10 days' WHERE id = (SELECT MIN(id) FROM outbox_events)`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE outbox_events SET published_at = now() - interval '1 day' WHERE id = (SELECT MIN(id) + 1 FROM outbox_events)`); err != nil {
		t.Fatal(err)
	}

	deleted, err := f.repo.DeletePublishedBefore(ctx, f.pool, 7*24*time.Hour)
	if err != nil || deleted != 1 {
		t.Fatalf("deleted = %d, %v", deleted, err)
	}
	if f.count(t, "TRUE") != 2 {
		t.Fatal("recent and unpublished events must survive")
	}
}
