package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/events"
)

type deleteRecorder struct {
	ids []uuid.UUID
	err error
}

func (r *deleteRecorder) DeleteAccount(_ context.Context, id uuid.UUID) error {
	r.ids = append(r.ids, id)
	return r.err
}

func authEvent(t *testing.T, typ string, p events.AccountDeletedPayload) []byte {
	t.Helper()
	b, err := json.Marshal(events.NewEvent(typ, p))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestHandleAuthEvent(t *testing.T) {
	userID := uuid.New()
	r := &deleteRecorder{}

	if err := HandleAuthEvent(context.Background(), r, authEvent(t, events.TypeAccountDeleted, events.AccountDeletedPayload{UserID: userID})); err != nil || len(r.ids) != 1 || r.ids[0] != userID {
		t.Fatalf("account_deleted: %v %v", r.ids, err)
	}
	for name, raw := range map[string][]byte{
		"other event": authEvent(t, "user.registered", events.AccountDeletedPayload{UserID: userID}),
		"empty user":  authEvent(t, events.TypeAccountDeleted, events.AccountDeletedPayload{}),
		"not json":    []byte("bad"),
	} {
		if err := HandleAuthEvent(context.Background(), r, raw); err != nil || len(r.ids) != 1 {
			t.Errorf("%s must be skipped: calls=%d err=%v", name, len(r.ids), err)
		}
	}

	r.err = errors.New("db down")
	if err := HandleAuthEvent(context.Background(), r, authEvent(t, events.TypeAccountDeleted, events.AccountDeletedPayload{UserID: userID})); err == nil {
		t.Fatal("db error must be returned for retry")
	}
}
