package grpcserver_test

import (
	"bytes"
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	conversationv1 "github.com/maltira/chavo-project-backend/proto/gen/go/conversation/v1"
	"github.com/maltira/chavo-project-backend/proto/grpcx"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/grpcserver"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/testutil"
	"github.com/maltira/chavo-project-backend/services/conversation-service/pkg/crypto"
)

type allowAllUsers struct{}

func (allowAllUsers) CheckMessagingAllowed(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (allowAllUsers) UserExists(context.Context, uuid.UUID) (bool, error)               { return true, nil }
func (allowAllUsers) GroupInviteAllowed(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}

type clients struct {
	api      conversationv1.ConversationServiceClient
	internal conversationv1.ConversationInternalServiceClient
}

func start(t *testing.T) clients {
	t.Helper()
	pool := testutil.Pool(t, "grpcserver_test")
	cipher, _ := crypto.NewCipher(bytes.Repeat([]byte{5}, 32))
	db := repository.NewDB(pool)
	cr, mr, or := repository.NewConversationRepository(), repository.NewMessageRepository(), repository.NewOutboxRepository()
	br, jr := repository.NewBanRepository(), repository.NewJoinRequestRepository()
	users := allowAllUsers{}
	log := zap.NewNop()

	impl := grpcserver.New(
		service.NewConversationService(db, cr, cipher),
		service.NewGroupService(db, cr, or, users, br),
		service.NewJoinService(db, cr, jr, or, br),
		service.NewMessageService(db, cr, mr, or, users, cipher, log),
	)
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer(grpc.UnaryInterceptor(grpcserver.UnaryInterceptor(log)))
	conversationv1.RegisterConversationServiceServer(srv, impl)
	conversationv1.RegisterConversationInternalServiceServer(srv, impl)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return clients{api: conversationv1.NewConversationServiceClient(conn), internal: conversationv1.NewConversationInternalServiceClient(conn)}
}

func as(u uuid.UUID) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), grpcx.MDUserID, u.String())
}

func str(s string) *string { return &s }

func expect(t *testing.T, err error, code codes.Code, reason string) {
	t.Helper()
	gotCode, gotReason, msg := grpcx.Details(err)
	if gotCode != code || gotReason != reason {
		t.Fatalf("got %v/%q (%q), want %v/%q", gotCode, gotReason, msg, code, reason)
	}
	if code != codes.OK && msg == "" {
		t.Fatalf("error without user message: %v", err)
	}
}

func TestIdentityAndParamValidation(t *testing.T) {
	c := start(t)
	u := uuid.New()

	_, err := c.api.ListConversations(context.Background(), &conversationv1.ListConversationsRequest{})
	expect(t, err, codes.Unauthenticated, "")
	_, err = c.api.GetConversation(as(u), &conversationv1.GetConversationRequest{ConversationId: "not-a-uuid"})
	expect(t, err, codes.InvalidArgument, "")
	_, err = c.api.ListMessages(as(u), &conversationv1.ListMessagesRequest{ConversationId: uuid.NewString(), Before: str("nope")})
	expect(t, err, codes.InvalidArgument, "")
	_, err = c.api.SendMessage(as(u), &conversationv1.SendMessageRequest{RecipientId: str(uuid.NewString())})
	expect(t, err, codes.InvalidArgument, "")
	_, err = c.api.GetConversation(as(u), &conversationv1.GetConversationRequest{ConversationId: uuid.NewString()})
	expect(t, err, codes.NotFound, "")
}

func TestMessageFlowAndPagination(t *testing.T) {
	c := start(t)
	a, b, outsider := uuid.New(), uuid.New(), uuid.New()

	first, err := c.api.SendMessage(as(a), &conversationv1.SendMessageRequest{RecipientId: str(b.String()), Content: "m1"})
	if err != nil {
		t.Fatal(err)
	}
	conv := first.GetMessage().GetConversationId()
	for _, text := range []string{"m2", "m3"} {
		if _, err = c.api.SendMessage(as(a), &conversationv1.SendMessageRequest{ConversationId: str(conv), Content: text}); err != nil {
			t.Fatal(err)
		}
	}

	page1, err := c.api.ListMessages(as(b), &conversationv1.ListMessagesRequest{ConversationId: conv, Limit: 2})
	if err != nil || len(page1.GetItems()) != 2 || page1.NextBefore == nil || page1.GetItems()[0].GetContent() != "m3" {
		t.Fatalf("page 1: %v %v", page1, err)
	}
	page2, err := c.api.ListMessages(as(b), &conversationv1.ListMessagesRequest{ConversationId: conv, Limit: 2, Before: page1.NextBefore})
	if err != nil || len(page2.GetItems()) != 1 || page2.NextBefore != nil {
		t.Fatalf("page 2: %v %v", page2, err)
	}

	_, err = c.api.ListMessages(as(outsider), &conversationv1.ListMessagesRequest{ConversationId: conv})
	expect(t, err, codes.PermissionDenied, "NOT_MEMBER")

	deleted, err := c.api.DeleteMessage(as(a), &conversationv1.DeleteMessageRequest{MessageId: first.GetMessage().GetId()})
	if err != nil || deleted == nil {
		t.Fatal(err)
	}
	got, err := c.api.GetMessage(as(b), &conversationv1.GetMessageRequest{MessageId: first.GetMessage().GetId()})
	if err != nil || !got.GetMessage().GetIsDeleted() || got.GetMessage().Content != nil {
		t.Fatalf("deleted message: %v %v", got, err)
	}

	peers, err := c.internal.ListDirectPeers(context.Background(), &conversationv1.ListDirectPeersRequest{UserId: a.String()})
	if err != nil || len(peers.GetUserIds()) != 1 || peers.GetUserIds()[0] != b.String() {
		t.Fatalf("direct peers: %v %v", peers, err)
	}
}

func TestGroupErrorsAndBan(t *testing.T) {
	c := start(t)
	admin, member := uuid.New(), uuid.New()

	created, err := c.api.CreateGroup(as(admin), &conversationv1.CreateGroupRequest{Name: "g", Visibility: "public", MemberIds: []string{member.String()}})
	if err != nil || created.InviteToken != nil {
		t.Fatalf("create public group: %v %v", created, err)
	}
	conv := created.GetConversation().GetId()

	_, err = c.api.RemoveMember(as(admin), &conversationv1.RemoveMemberRequest{ConversationId: conv, UserId: admin.String()})
	expect(t, err, codes.FailedPrecondition, "LAST_ADMIN")
	_, err = c.api.RemoveMember(as(admin), &conversationv1.RemoveMemberRequest{ConversationId: conv, UserId: admin.String(), Ban: true})
	expect(t, err, codes.InvalidArgument, "")
	_, err = c.api.ListBans(as(member), &conversationv1.ListBansRequest{ConversationId: conv})
	expect(t, err, codes.PermissionDenied, "")

	if _, err = c.api.RemoveMember(as(admin), &conversationv1.RemoveMemberRequest{ConversationId: conv, UserId: member.String(), Ban: true}); err != nil {
		t.Fatal(err)
	}
	_, err = c.api.JoinGroup(as(member), &conversationv1.JoinGroupRequest{ConversationId: conv})
	expect(t, err, codes.PermissionDenied, "USER_BANNED")

	bans, err := c.api.ListBans(as(admin), &conversationv1.ListBansRequest{ConversationId: conv})
	if err != nil || len(bans.GetItems()) != 1 || bans.GetItems()[0].GetUserId() != member.String() {
		t.Fatalf("bans: %v %v", bans, err)
	}
	if _, err = c.api.Unban(as(admin), &conversationv1.UnbanRequest{ConversationId: conv, UserId: member.String()}); err != nil {
		t.Fatal(err)
	}
	if _, err = c.api.JoinGroup(as(member), &conversationv1.JoinGroupRequest{ConversationId: conv}); err != nil {
		t.Fatalf("join after unban: %v", err)
	}

	private, err := c.api.CreateGroup(as(admin), &conversationv1.CreateGroupRequest{Name: "p"})
	if err != nil || private.InviteToken == nil || private.GetConversation().GetVisibility() != "private" {
		t.Fatalf("private group must return invite token once: %v %v", private, err)
	}
}
