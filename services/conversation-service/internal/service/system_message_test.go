package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
)

// systemMessages - системные сообщения, видимые участнику, от старых к новым
func systemMessages(t *testing.T, e *env, user, conv uuid.UUID) []models.Message {
	t.Helper()
	list, err := e.msgs.List(context.Background(), user, conv, nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	res := make([]models.Message, 0)
	for i := len(list) - 1; i >= 0; i-- {
		if list[i].Kind == models.MessageSystem {
			res = append(res, list[i])
		}
	}
	return res
}

func assertSystem(t *testing.T, m models.Message, event string, actor, target uuid.UUID) {
	t.Helper()
	if m.SystemEvent == nil || *m.SystemEvent != event || m.SenderID != actor || m.TargetID == nil ||
		*m.TargetID != target || m.Content != nil {
		t.Fatalf("system message = %+v, want %s %s -> %s", m, event, actor, target)
	}
}

// системные события в outbox: message.created с kind=system по этому чату
func systemEvents(t *testing.T, e *env, conv uuid.UUID) int {
	t.Helper()
	return count(t, e.pool, `SELECT COUNT(*) FROM outbox_events
		WHERE payload->>'event_type' = 'message.created'
		  AND payload->'payload'->>'conversation_id' = $1
		  AND payload->'payload'->>'kind' = 'system'`, conv.String())
}

func TestSystemMessagesForMembership(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, added, joiner, kicked := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	conv := createGroup(t, e, admin, "public").Conversation.ID

	if err := e.groups.AddMembers(ctx, admin, conv, []uuid.UUID{added, kicked}); err != nil {
		t.Fatal(err)
	}
	if err := e.groups.Join(ctx, joiner, conv); err != nil {
		t.Fatal(err)
	}
	if err := e.groups.RemoveMember(ctx, joiner, conv, joiner, false); err != nil {
		t.Fatal(err)
	}
	// бан в истории - обычное исключение
	if err := e.groups.RemoveMember(ctx, admin, conv, kicked, true); err != nil {
		t.Fatal(err)
	}

	got := systemMessages(t, e, admin, conv)
	if len(got) != 5 {
		t.Fatalf("system messages = %d, want 5: %+v", len(got), got)
	}
	assertSystem(t, got[0], models.SystemAdded, admin, added)
	assertSystem(t, got[1], models.SystemAdded, admin, kicked)
	assertSystem(t, got[2], models.SystemJoined, joiner, joiner)
	assertSystem(t, got[3], models.SystemLeft, joiner, joiner)
	assertSystem(t, got[4], models.SystemRemoved, admin, kicked)
	if n := systemEvents(t, e, conv); n != 5 {
		t.Fatalf("system events in outbox = %d, want 5", n)
	}

	// чат поднимается, последнее сообщение - системное, но счётчик непрочитанных не растёт
	sum, err := e.convs.Get(ctx, added, conv)
	if err != nil || sum.UnreadCount != 0 || sum.LastMessage == nil || sum.LastMessage.ID != got[4].ID ||
		!sum.LastMessageAt.Equal(got[4].CreatedAt) {
		t.Fatalf("summary: %+v %v", sum, err)
	}
}

func TestSystemMessageForApprovedRequest(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, guest := uuid.New(), uuid.New()
	g := createGroup(t, e, admin, "private")
	req, err := e.joins.RequestJoin(ctx, guest, g.Conversation.ID, *g.InviteToken)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.joins.Approve(ctx, admin, g.Conversation.ID, req.ID); err != nil {
		t.Fatal(err)
	}
	got := systemMessages(t, e, guest, g.Conversation.ID)
	if len(got) != 1 {
		t.Fatalf("guest sees %d system messages, want 1", len(got))
	}
	assertSystem(t, got[0], models.SystemJoined, guest, guest)
}

func TestSystemMessagesAreReadOnly(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, member := uuid.New(), uuid.New()
	conv := createGroup(t, e, admin, "public").Conversation.ID
	if err := e.groups.AddMembers(ctx, admin, conv, []uuid.UUID{member}); err != nil {
		t.Fatal(err)
	}
	sys := systemMessages(t, e, admin, conv)[0].ID

	if _, err := e.msgs.Edit(ctx, admin, sys, "текст"); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("edit system: %v", err)
	}
	if err := e.msgs.Delete(ctx, admin, sys); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("delete system: %v", err)
	}
	if _, err := e.msgs.Send(ctx, member, service.SendMessageInput{
		ConversationID: &conv, Content: "ответ", ReplyToMessageID: &sys,
	}); !errors.Is(err, apperror.ErrInvalidReply) {
		t.Fatalf("reply to system: %v", err)
	}
	// прочитать до системного сообщения можно - это обычный курсор
	if err := e.msgs.MarkRead(ctx, member, conv, sys); err != nil {
		t.Fatalf("mark read system: %v", err)
	}
}
