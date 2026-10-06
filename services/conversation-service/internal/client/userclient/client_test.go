package userclient

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	userv1 "github.com/maltira/chavo-project-backend/proto/gen/go/user/v1"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
)

type fakeUserService struct {
	userv1.UnimplementedUserInternalServiceServer
	messaging *userv1.MessagingAllowedResponse
	inviteErr error
	lastReq   *userv1.GroupInviteAllowedRequest
	failAll   bool
}

func (f *fakeUserService) MessagingAllowed(context.Context, *userv1.MessagingAllowedRequest) (*userv1.MessagingAllowedResponse, error) {
	if f.failAll {
		return nil, status.Error(codes.Unavailable, "down")
	}
	return f.messaging, nil
}

func (f *fakeUserService) UserExists(context.Context, *userv1.UserExistsRequest) (*userv1.UserExistsResponse, error) {
	if f.failAll {
		return nil, status.Error(codes.Internal, "boom")
	}
	return &userv1.UserExistsResponse{Exists: true}, nil
}

func (f *fakeUserService) GroupInviteAllowed(_ context.Context, req *userv1.GroupInviteAllowedRequest) (*userv1.GroupInviteAllowedResponse, error) {
	f.lastReq = req
	if f.inviteErr != nil {
		return nil, f.inviteErr
	}
	return &userv1.GroupInviteAllowedResponse{Allowed: false}, nil
}

func newClient(t *testing.T, fake *fakeUserService) Client {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	userv1.RegisterUserInternalServiceServer(srv, fake)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return New(conn)
}

func TestCheckMessagingAllowed(t *testing.T) {
	cases := map[string]struct {
		resp *userv1.MessagingAllowedResponse
		want error
	}{
		"allowed":      {&userv1.MessagingAllowedResponse{Allowed: true}, nil},
		"by sender":    {&userv1.MessagingAllowedResponse{BlockedBySender: true}, apperror.ErrBlockedByMe},
		"by recipient": {&userv1.MessagingAllowedResponse{BlockedByRecipient: true}, apperror.ErrBlockedByThem},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := newClient(t, &fakeUserService{messaging: tc.resp}).CheckMessagingAllowed(context.Background(), uuid.New(), uuid.New())
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestUserServiceFailure(t *testing.T) {
	c := newClient(t, &fakeUserService{failAll: true})
	if _, err := c.UserExists(context.Background(), uuid.New()); !errors.Is(err, apperror.ErrUserServiceError) {
		t.Fatalf("UserExists: %v", err)
	}
	if err := c.CheckMessagingAllowed(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, apperror.ErrUserServiceError) {
		t.Fatalf("CheckMessagingAllowed: %v", err)
	}
}

func TestGroupInviteAllowed(t *testing.T) {
	fake := &fakeUserService{}
	c := newClient(t, fake)
	user, inviter := uuid.New(), uuid.New()

	allowed, err := c.GroupInviteAllowed(context.Background(), user, inviter)
	if err != nil || allowed || fake.lastReq.GetInviterId() != inviter.String() || fake.lastReq.GetUserId() != user.String() {
		t.Fatalf("allowed=%v err=%v req=%v", allowed, err, fake.lastReq)
	}

	fake.inviteErr = status.Error(codes.NotFound, "no user")
	if _, err = c.GroupInviteAllowed(context.Background(), user, inviter); !errors.Is(err, apperror.ErrUserNotFound) {
		t.Fatalf("not found: %v", err)
	}
}
