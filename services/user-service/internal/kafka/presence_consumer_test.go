package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/events"
)

type recorder struct {
	calls []time.Time
	err   error
}

func (r *recorder) UpdateLastSeen(_ context.Context, _ uuid.UUID, at time.Time) error {
	r.calls = append(r.calls, at)
	return r.err
}

func event(t *testing.T, typ string, p events.PresencePayload) []byte {
	t.Helper()
	b, err := json.Marshal(events.NewEvent(typ, p))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestHandle(t *testing.T) {
	at := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	user := uuid.New()
	r := &recorder{}

	if err := Handle(context.Background(), r, event(t, events.TypeUserOffline, events.PresencePayload{UserID: user, At: at})); err != nil || len(r.calls) != 1 || !r.calls[0].Equal(at) {
		t.Fatalf("offline: %v %v", r.calls, err)
	}
	for name, raw := range map[string][]byte{
		"online":   event(t, events.TypeUserOnline, events.PresencePayload{UserID: user, At: at}),
		"no user":  event(t, events.TypeUserOffline, events.PresencePayload{At: at}),
		"no time":  event(t, events.TypeUserOffline, events.PresencePayload{UserID: user}),
		"not json": []byte("garbage"),
	} {
		if err := Handle(context.Background(), r, raw); err != nil || len(r.calls) != 1 {
			t.Errorf("%s must be skipped: calls=%d err=%v", name, len(r.calls), err)
		}
	}

	r.err = errors.New("db down")
	if err := Handle(context.Background(), r, event(t, events.TypeUserOffline, events.PresencePayload{UserID: user, At: at})); err == nil {
		t.Fatal("db error must be returned for retry")
	}
}
