package outbox_test

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/google/uuid"

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

func TestDecryptFailureBlocksQueue(t *testing.T) {
	f := setup(t)
	f.insertMessageEvent(t, "message-events", map[string]any{"message_id": "bad", "content_enc": []byte("garbage-garbage-garbage-garbage")})
	f.insertMessageEvent(t, "message-events", map[string]any{"message_id": "next"})

	pub := &rawPub{}
	n, err := f.publisher(pub, 10).RunOnce(context.Background())
	if err == nil || n != 0 || len(pub.msgs) != 0 {
		t.Fatalf("RunOnce = %d, %v, sent %d", n, err, len(pub.msgs))
	}
	if f.count(t, "attempts = 1 AND published_at IS NULL") != 1 || f.count(t, "attempts = 0 AND published_at IS NULL") != 1 {
		t.Fatal("failed event gets an attempt, later event stays untouched")
	}
}
