package ws

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	conversationv1 "github.com/maltira/chavo-project-backend/proto/gen/go/conversation/v1"
)

type fakeTypingSink struct {
	mu   sync.Mutex
	sent []struct {
		users []string
		msg   []byte
	}
}

func (f *fakeTypingSink) SendToUsers(ids []string, msg []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, struct {
		users []string
		msg   []byte
	}{users: append([]string(nil), ids...), msg: append([]byte(nil), msg...)})
}

type fakeConvInternal struct {
	conversationv1.ConversationInternalServiceClient
	mu      sync.Mutex
	members map[string][]string
	calls   int
	err     error
}

func (f *fakeConvInternal) ListMemberIDs(_ context.Context, req *conversationv1.ListMemberIDsRequest, _ ...grpc.CallOption) (*conversationv1.ListMemberIDsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	ids, ok := f.members[req.GetConversationId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "not found")
	}
	if !slices.Contains(ids, req.GetUserId()) {
		return nil, status.Error(codes.PermissionDenied, "not member")
	}
	return &conversationv1.ListMemberIDsResponse{UserIds: ids}, nil
}

func TestTypingServiceBroadcastThrottleCacheAndInvalidate(t *testing.T) {
	convID := uuid.NewString()
	u1, u2, u3 := uuid.NewString(), uuid.NewString(), uuid.NewString()

	sink := &fakeTypingSink{}
	convs := &fakeConvInternal{members: map[string][]string{convID: {u1, u2, u3}}}
	svc := NewTypingService(sink, convs, zap.NewNop())

	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	// невалидный UUID отклоняется
	if err := svc.Notify(context.Background(), u1, "bad-id"); err != ErrInvalidConversationID {
		t.Fatalf("bad id: %v", err)
	}

	// первый вызов идёт в gRPC и рассылает остальным участникам (кроме отправителя)
	if err := svc.Notify(context.Background(), u1, convID); err != nil {
		t.Fatal(err)
	}
	if convs.calls != 1 || len(sink.sent) != 1 || !slices.Equal(sink.sent[0].users, []string{u2, u3}) {
		t.Fatalf("first notify: calls=%d sent=%+v", convs.calls, sink.sent)
	}
	var frame map[string]any
	if err := json.Unmarshal(sink.sent[0].msg, &frame); err != nil {
		t.Fatal(err)
	}
	data := frame["data"].(map[string]any)
	if frame["type"] != "conversation.typing" || data["conversation_id"] != convID || data["user_id"] != u1 {
		t.Fatalf("frame: %+v", frame)
	}

	// повтор в пределах minInterval (2с) молча игнорируется
	now = now.Add(time.Second)
	if err := svc.Notify(context.Background(), u1, convID); err != nil {
		t.Fatal(err)
	}
	if convs.calls != 1 || len(sink.sent) != 1 {
		t.Fatalf("throttled: calls=%d sent=%d", convs.calls, len(sink.sent))
	}

	// через 3с рассылка проходит, но список участников берётся из кеша (без нового gRPC)
	now = now.Add(2 * time.Second)
	if err := svc.Notify(context.Background(), u1, convID); err != nil {
		t.Fatal(err)
	}
	if convs.calls != 1 || len(sink.sent) != 2 {
		t.Fatalf("cached: calls=%d sent=%d", convs.calls, len(sink.sent))
	}

	// после Invalidate список заново читается из gRPC
	svc.Invalidate(convID)
	convs.members[convID] = []string{u1, u2}
	now = now.Add(3 * time.Second)
	if err := svc.Notify(context.Background(), u1, convID); err != nil {
		t.Fatal(err)
	}
	if convs.calls != 2 || len(sink.sent) != 3 || !slices.Equal(sink.sent[2].users, []string{u2}) {
		t.Fatalf("after invalidate: calls=%d sent=%+v", convs.calls, sink.sent)
	}

	// посторонний не рассылает ничего и не получает ошибку
	outsider := uuid.NewString()
	if err := svc.Notify(context.Background(), outsider, convID); err != nil {
		t.Fatalf("outsider: %v", err)
	}
	if len(sink.sent) != 3 {
		t.Fatalf("outsider sent: %d", len(sink.sent))
	}
}
