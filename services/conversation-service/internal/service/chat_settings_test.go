package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/proto/mute"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
)

// direct создаёт личный чат me с новым собеседником и возвращает его id
func direct(t *testing.T, e *env, me uuid.UUID) uuid.UUID {
	t.Helper()
	peer := uuid.New()
	m, err := e.msgs.Send(context.Background(), peer, service.SendMessageInput{RecipientID: &me, Content: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	return m.ConversationID
}

func listIDs(t *testing.T, e *env, user uuid.UUID, archived bool) []uuid.UUID {
	t.Helper()
	items, err := e.convs.List(context.Background(), user, archived, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]uuid.UUID, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	return ids
}

func settingsEvents(t *testing.T, e *env, convID uuid.UUID) int {
	return count(t, e.pool, `SELECT COUNT(*) FROM outbox_events
		WHERE payload->>'event_type' = 'conversation.settings.updated' AND event_key = $1`, convID.String())
}

func TestChatMute(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	me := uuid.New()
	id := direct(t, e, me)

	bad := "3h"
	if _, err := e.convs.SetMute(ctx, me, id, &bad); !errors.Is(err, mute.ErrUnknownDuration) {
		t.Fatalf("unknown duration: %v", err)
	}
	if _, err := e.convs.SetMute(ctx, uuid.New(), id, nil); !errors.Is(err, apperror.ErrNotMember) {
		t.Fatalf("outsider: %v", err)
	}

	d := "8h"
	cs, err := e.convs.SetMute(ctx, me, id, &d)
	if err != nil || cs.MutedUntil == nil || time.Until(*cs.MutedUntil) < 7*time.Hour {
		t.Fatalf("mute 8h: %+v %v", cs, err)
	}
	forever := "forever"
	if cs, err = e.convs.SetMute(ctx, me, id, &forever); err != nil || !cs.MutedUntil.Equal(mute.Forever) {
		t.Fatalf("mute forever: %+v %v", cs, err)
	}
	got, err := e.convs.Get(ctx, me, id)
	if err != nil || got.MutedUntil == nil || !got.MutedUntil.Equal(mute.Forever) {
		t.Fatalf("get: %+v %v", got, err)
	}

	// настройка личная: у собеседника звук не выключен
	peer := *got.PeerID
	if other, _ := e.convs.Get(ctx, peer, id); other.MutedUntil != nil {
		t.Fatalf("peer sees my mute: %v", other.MutedUntil)
	}

	if cs, err = e.convs.SetMute(ctx, me, id, nil); err != nil || cs.MutedUntil != nil {
		t.Fatalf("unmute: %+v %v", cs, err)
	}
	// событие только мне, по одному на изменение
	if n := settingsEvents(t, e, id); n != 3 {
		t.Fatalf("events = %d, want 3", n)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM outbox_events
		WHERE payload->>'event_type' = 'conversation.settings.updated'
		  AND payload->'payload'->'member_ids' <> to_jsonb(ARRAY[$1::text])`, me.String()); n != 0 {
		t.Fatalf("%d events addressed to others", n)
	}
}

func TestChatArchive(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	me := uuid.New()
	a, b := direct(t, e, me), direct(t, e, me)

	if _, err := e.convs.SetPinned(ctx, me, a, true); err != nil {
		t.Fatal(err)
	}
	cs, err := e.convs.SetArchived(ctx, me, a, true)
	if err != nil || cs.ArchivedAt == nil || cs.PinnedAt != nil {
		t.Fatalf("archive must unpin: %+v %v", cs, err)
	}
	if ids := listIDs(t, e, me, false); len(ids) != 1 || ids[0] != b {
		t.Fatalf("main list = %v", ids)
	}
	if ids := listIDs(t, e, me, true); len(ids) != 1 || ids[0] != a {
		t.Fatalf("archive = %v", ids)
	}

	// новое сообщение не возвращает чат из архива; повторная архивация ничего не меняет
	got, _ := e.convs.Get(ctx, me, a)
	if _, err := e.msgs.Send(ctx, *got.PeerID, service.SendMessageInput{ConversationID: &a, Content: "ещё"}); err != nil {
		t.Fatal(err)
	}
	if ids := listIDs(t, e, me, true); len(ids) != 1 || ids[0] != a {
		t.Fatalf("archive after message = %v", ids)
	}
	before := settingsEvents(t, e, a)
	if _, err := e.convs.SetArchived(ctx, me, a, true); err != nil || settingsEvents(t, e, a) != before {
		t.Fatalf("repeat archive: %v", err)
	}

	// закреп достаёт из архива
	if cs, err = e.convs.SetPinned(ctx, me, a, true); err != nil || cs.ArchivedAt != nil || cs.PinnedAt == nil {
		t.Fatalf("pin from archive: %+v %v", cs, err)
	}
	if ids := listIDs(t, e, me, true); len(ids) != 0 {
		t.Fatalf("archive after pin = %v", ids)
	}
	if cs, err = e.convs.SetArchived(ctx, me, b, true); err != nil {
		t.Fatal(err)
	}
	if cs, err = e.convs.SetArchived(ctx, me, b, false); err != nil || cs.ArchivedAt != nil {
		t.Fatalf("unarchive: %+v %v", cs, err)
	}
}

func TestChatPinOrderAndLimit(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	me := uuid.New()
	ids := make([]uuid.UUID, 7)
	for i := range ids {
		ids[i] = direct(t, e, me)
	}
	// без закрепов - по свежести: последний созданный сверху
	if got := listIDs(t, e, me, false); got[0] != ids[6] {
		t.Fatalf("order = %v", got)
	}

	if _, err := e.convs.SetPinned(ctx, me, ids[0], true); err != nil {
		t.Fatal(err)
	}
	if _, err := e.convs.SetPinned(ctx, me, ids[1], true); err != nil {
		t.Fatal(err)
	}
	got := listIDs(t, e, me, false)
	if got[0] != ids[1] || got[1] != ids[0] || got[2] != ids[6] {
		t.Fatalf("pinned first, newest pin on top: %v", got)
	}
	// повторный закреп не двигает чат
	first, _ := e.convs.Get(ctx, me, ids[0])
	again, err := e.convs.SetPinned(ctx, me, ids[0], true)
	if err != nil || !again.PinnedAt.Equal(*first.PinnedAt) {
		t.Fatalf("repeat pin: %+v %v", again, err)
	}

	// лимит под параллельными запросами
	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for _, id := range ids[2:] {
		wg.Add(1)
		go func(id uuid.UUID) {
			defer wg.Done()
			_, err := e.convs.SetPinned(ctx, me, id, true)
			errs <- err
		}(id)
	}
	wg.Wait()
	close(errs)
	limited := 0
	for err := range errs {
		switch {
		case errors.Is(err, apperror.ErrPinLimit):
			limited++
		case err != nil:
			t.Fatalf("pin: %v", err)
		}
	}
	if limited != 2 {
		t.Fatalf("limited = %d, want 2", limited)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM conversation_members WHERE user_id = $1 AND pinned_at IS NOT NULL`, me); n != 5 {
		t.Fatalf("pinned = %d", n)
	}

	var cs *models.ChatSettings
	if cs, err = e.convs.SetPinned(ctx, me, ids[0], false); err != nil || cs.PinnedAt != nil {
		t.Fatalf("unpin: %+v %v", cs, err)
	}
}
