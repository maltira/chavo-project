package outbox_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/outbox"
	"github.com/maltira/chavo-project-backend/services/conversation-service/pkg/crypto"
)

func testCipher() *crypto.Cipher {
	c, err := crypto.NewCipher(bytes.Repeat([]byte{9}, 32))
	if err != nil {
		panic(err)
	}
	return c
}

type rawPub struct {
	mu   sync.Mutex
	msgs [][]byte
}

func (r *rawPub) Publish(_ context.Context, _, _ string, event any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, event.(json.RawMessage))
	return nil
}

func (f *fixture) insertMessageEvent(t *testing.T, topic string, payload map[string]any) {
	t.Helper()
	event := map[string]any{"event_id": uuid.NewString(), "event_type": "message.created", "payload": payload}
	if err := f.repo.Insert(context.Background(), f.pool, topic, "conv", event); err != nil {
		t.Fatal(err)
	}
}

func TestPublisherDecryptsMessageContent(t *testing.T) {
	f := setup(t)
	enc, _ := testCipher().Encrypt([]byte("привет, мир"))
	f.insertMessageEvent(t, "message-events", map[string]any{"message_id": "m1", "content_enc": enc})

	pub := &rawPub{}
	if n, err := f.publisher(pub, 10).RunOnce(context.Background()); err != nil || n != 1 {
		t.Fatalf("RunOnce = %d, %v", n, err)
	}

	var got struct {
		EventID string `json:"event_id"`
		Payload map[string]any
	}
	if err := json.Unmarshal(pub.msgs[0], &got); err != nil {
		t.Fatal(err)
	}
	if got.EventID == "" || got.Payload["content"] != "привет, мир" || got.Payload["message_id"] != "m1" {
		t.Fatalf("payload = %v", got.Payload)
	}
	if _, ok := got.Payload["content_enc"]; ok {
		t.Fatal("content_enc must not reach Kafka")
	}
}

func TestPublisherPassesThroughWithoutContent(t *testing.T) {
	f := setup(t)
	f.insertMessageEvent(t, "message-events", map[string]any{"message_id": "m1"})
	enc, _ := testCipher().Encrypt([]byte("x"))
	f.insertMessageEvent(t, "conversation-events", map[string]any{"content_enc": enc})

	pub := &rawPub{}
	if n, err := f.publisher(pub, 10).RunOnce(context.Background()); err != nil || n != 2 {
		t.Fatalf("RunOnce = %d, %v", n, err)
	}
	if bytes.Contains(pub.msgs[0], []byte(`"content"`)) {
		t.Fatalf("scrubbed event must have no content: %s", pub.msgs[0])
	}
	if !bytes.Contains(pub.msgs[1], []byte(`"content_enc"`)) {
		t.Fatalf("other topics must not be touched: %s", pub.msgs[1])
	}
}

func (f *fixture) publisherWith(pub outbox.EventPublisher, dec outbox.Decrypter) *outbox.Publisher {
	opts := outbox.DefaultOptions()
	opts.PollInterval = 10 * time.Millisecond
	opts.PublishTimeout = time.Second
	return outbox.NewPublisher(f.db, f.repo, pub, dec, zap.NewNop(), opts)
}

type failingPub struct{}

func (failingPub) Publish(context.Context, string, string, any) error {
	return errors.New("kafka down")
}

func TestPoisonEventIsParkedAndQueueContinues(t *testing.T) {
	f := setup(t)
	f.insertMessageEvent(t, "message-events", map[string]any{"message_id": "bad", "content_enc": []byte("garbage-garbage-garbage-garbage")})
	f.insertMessageEvent(t, "message-events", map[string]any{"message_id": "next"})

	pub := &rawPub{}
	p := f.publisher(pub, 10)
	for attempt := 1; attempt <= 2; attempt++ {
		n, err := p.RunOnce(context.Background())
		if err == nil || n != 0 || len(pub.msgs) != 0 {
			t.Fatalf("attempt %d: RunOnce = %d, %v, sent %d", attempt, n, err, len(pub.msgs))
		}
		if f.count(t, fmt.Sprintf("failed_at IS NULL AND published_at IS NULL AND attempts = %d", attempt)) != 1 {
			t.Fatalf("attempt %d: failed event must get attempts=%d and stay in the queue", attempt, attempt)
		}
	}

	n, err := p.RunOnce(context.Background())
	if err != nil || n != 1 || len(pub.msgs) != 1 || !bytes.Contains(pub.msgs[0], []byte(`"next"`)) {
		t.Fatalf("third attempt: RunOnce = %d, %v, sent %d", n, err, len(pub.msgs))
	}
	if f.count(t, "failed_at IS NOT NULL AND published_at IS NULL AND attempts = 3") != 1 {
		t.Fatal("poison event must be parked after 3 attempts")
	}
	if n, err = p.RunOnce(context.Background()); err != nil || n != 0 {
		t.Fatalf("parked event must be skipped: %d, %v", n, err)
	}
}

func TestKafkaFailuresNeverParkEvents(t *testing.T) {
	f := setup(t)
	f.insertMessageEvent(t, "message-events", map[string]any{"message_id": "m1"})

	p := f.publisher(failingPub{}, 10)
	for i := 0; i < 6; i++ {
		if _, err := p.RunOnce(context.Background()); err == nil {
			t.Fatal("expected a publish error")
		}
	}
	if f.count(t, "failed_at IS NOT NULL") != 0 || f.count(t, "attempts = 6 AND published_at IS NULL") != 1 {
		t.Fatal("a broker outage must not park events")
	}
}

func TestParkedEventCanBeReplayed(t *testing.T) {
	f := setup(t)
	other, _ := crypto.NewCipher(bytes.Repeat([]byte{3}, 32))
	enc, _ := other.Encrypt([]byte("после починки"))
	f.insertMessageEvent(t, "message-events", map[string]any{"message_id": "m1", "content_enc": enc})

	wrongKey := f.publisher(&rawPub{}, 10) // ключ 9 не подходит к шифртексту ключа 3
	for i := 0; i < 3; i++ {
		_, _ = wrongKey.RunOnce(context.Background())
	}
	if f.count(t, "failed_at IS NOT NULL") != 1 {
		t.Fatal("event must be parked")
	}

	if _, err := f.pool.Exec(context.Background(), `UPDATE outbox_events SET failed_at = NULL, attempts = 0`); err != nil {
		t.Fatal(err)
	}
	pub := &rawPub{}
	if n, err := f.publisherWith(pub, other).RunOnce(context.Background()); err != nil || n != 1 {
		t.Fatalf("replay: %d, %v", n, err)
	}
	if !bytes.Contains(pub.msgs[0], []byte("после починки")) {
		t.Fatalf("replayed payload: %s", pub.msgs[0])
	}
}
