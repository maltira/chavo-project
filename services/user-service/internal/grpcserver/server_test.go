package grpcserver_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	userv1 "github.com/maltira/chavo-project-backend/proto/gen/go/user/v1"
	"github.com/maltira/chavo-project-backend/proto/grpcx"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/grpcserver"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/service"
)

type fakeProfiles struct {
	service.ProfileService
	byID      map[uuid.UUID]models.Profile
	createErr error
}

func (f *fakeProfiles) FindByID(_ context.Context, id uuid.UUID) (*models.Profile, error) {
	p, ok := f.byID[id]
	if !ok {
		return nil, apperror.ErrNotFound
	}
	return &p, nil
}

func (f *fakeProfiles) GetAllBySearch(context.Context, string, int, int) ([]models.Profile, error) {
	res := make([]models.Profile, 0, len(f.byID))
	for _, p := range f.byID {
		res = append(res, p)
	}
	return res, nil
}

func (f *fakeProfiles) Create(context.Context, uuid.UUID, service.CreateProfileInput) error {
	return f.createErr
}

type fakeBlocks struct {
	service.BlockService
	blocked map[[2]uuid.UUID]bool // {blocker, blocked}
}

func (f *fakeBlocks) GetBlockStatus(_ context.Context, my, target uuid.UUID) (bool, bool, error) {
	return f.blocked[[2]uuid.UUID{my, target}], f.blocked[[2]uuid.UUID{target, my}], nil
}

type fakeSettings struct {
	service.SettingsService
	invites map[uuid.UUID]bool // наличие ключа = настройки есть
	show    map[uuid.UUID]bool
}

func (f *fakeSettings) GetSettings(_ context.Context, id uuid.UUID) (*models.Settings, error) {
	allowed, ok := f.invites[id]
	if !ok {
		return nil, apperror.ErrNotFound
	}
	return &models.Settings{UserID: id, AllowGroupInvites: allowed}, nil
}

func (f *fakeSettings) ShowOnlineStatus(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	res := map[uuid.UUID]bool{}
	for _, id := range ids {
		if v, ok := f.show[id]; ok {
			res[id] = v
		}
	}
	return res, nil
}

type fakePresence struct{ online map[uuid.UUID]bool }

func (f *fakePresence) Online(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	res := map[uuid.UUID]bool{}
	for _, id := range ids {
		res[id] = f.online[id]
	}
	return res, nil
}

type deps struct {
	profiles *fakeProfiles
	blocks   *fakeBlocks
	settings *fakeSettings
	presence *fakePresence
}

func newDeps() *deps {
	return &deps{
		profiles: &fakeProfiles{byID: map[uuid.UUID]models.Profile{}},
		blocks:   &fakeBlocks{blocked: map[[2]uuid.UUID]bool{}},
		settings: &fakeSettings{invites: map[uuid.UUID]bool{}, show: map[uuid.UUID]bool{}},
		presence: &fakePresence{online: map[uuid.UUID]bool{}},
	}
}

func start(t *testing.T, d *deps) (userv1.UserServiceClient, userv1.UserInternalServiceClient) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer(grpc.UnaryInterceptor(grpcserver.UnaryInterceptor(zap.NewNop())))
	impl := grpcserver.New(d.profiles, d.blocks, d.settings, d.presence)
	userv1.RegisterUserServiceServer(srv, impl)
	userv1.RegisterUserInternalServiceServer(srv, impl)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return userv1.NewUserServiceClient(conn), userv1.NewUserInternalServiceClient(conn)
}

func as(user uuid.UUID) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), grpcx.MDUserID, user.String())
}

func TestProfileOnlineVisibility(t *testing.T) {
	d := newDeps()
	viewer, hidden, open := uuid.New(), uuid.New(), uuid.New()
	seen := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, id := range []uuid.UUID{viewer, hidden, open} {
		d.profiles.byID[id] = models.Profile{UserID: id, Username: "u", DisplayName: "U", LastSeenAt: seen}
		d.presence.online[id] = true
	}
	d.settings.show[hidden] = false
	d.settings.show[open] = true
	d.settings.show[viewer] = false
	users, _ := start(t, d)

	got, err := users.GetProfile(as(viewer), &userv1.GetProfileRequest{UserId: hidden.String()})
	if err != nil || got.GetProfile().GetOnline() || got.GetProfile().GetLastSeenAt() != nil {
		t.Fatalf("hidden user leaks presence: %v %v", got, err)
	}
	got, err = users.GetProfile(as(viewer), &userv1.GetProfileRequest{UserId: open.String()})
	if err != nil || !got.GetProfile().GetOnline() || !got.GetProfile().GetLastSeenAt().AsTime().Equal(seen) {
		t.Fatalf("visible user: %v %v", got, err)
	}
	me, err := users.GetMe(as(viewer), &userv1.GetMeRequest{})
	if err != nil || !me.GetProfile().GetOnline() || me.GetProfile().GetLastSeenAt() == nil {
		t.Fatalf("own profile must show own presence: %v %v", me, err)
	}

	found, err := users.SearchProfiles(as(viewer), &userv1.SearchProfilesRequest{Query: "u"})
	if err != nil || len(found.GetProfiles()) != 3 {
		t.Fatalf("search: %v %v", found, err)
	}
	for _, p := range found.GetProfiles() {
		if want := p.GetUserId() != hidden.String(); p.GetOnline() != want {
			t.Errorf("search online for %s = %v, want %v", p.GetUserId(), p.GetOnline(), want)
		}
	}
}

func TestIdentityAndErrorMapping(t *testing.T) {
	d := newDeps()
	users, _ := start(t, d)

	_, err := users.GetMe(context.Background(), &userv1.GetMeRequest{})
	if code, _, msg := grpcx.Details(err); code != codes.Unauthenticated || msg == "" {
		t.Fatalf("no x-user-id: %v %q", code, msg)
	}

	d.profiles.createErr = apperror.ErrProfileAlreadyExists
	_, err = users.CreateProfile(as(uuid.New()), &userv1.CreateProfileRequest{Username: "abc", DisplayName: "A"})
	if code, reason, msg := grpcx.Details(err); code != codes.FailedPrecondition || reason != "PROFILE_ALREADY_EXISTS" || msg != "Профиль уже создан" {
		t.Fatalf("conflict mapping: %v %q %q", code, reason, msg)
	}
	if _, err = users.CreateProfile(as(uuid.New()), &userv1.CreateProfileRequest{Username: "abc"}); grpcxCode(err) != codes.InvalidArgument {
		t.Fatalf("missing display name: %v", err)
	}
	if _, err = users.GetProfile(as(uuid.New()), &userv1.GetProfileRequest{UserId: "nope"}); grpcxCode(err) != codes.InvalidArgument {
		t.Fatalf("bad uuid: %v", err)
	}
}

func grpcxCode(err error) codes.Code {
	code, _, _ := grpcx.Details(err)
	return code
}

func TestGroupInviteAllowed(t *testing.T) {
	d := newDeps()
	inviter, open, closed, blockedByInviter, blocksInviter := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	d.blocks.blocked[[2]uuid.UUID{inviter, blockedByInviter}] = true
	d.blocks.blocked[[2]uuid.UUID{blocksInviter, inviter}] = true
	d.settings.invites = map[uuid.UUID]bool{open: true, closed: false, blockedByInviter: true, blocksInviter: true}
	_, internal := start(t, d)

	for name, tc := range map[string]struct {
		user    uuid.UUID
		allowed bool
	}{
		"open":                     {open, true},
		"invites disabled":         {closed, false},
		"inviter blocked the user": {blockedByInviter, false},
		"user blocked the inviter": {blocksInviter, false},
	} {
		resp, err := internal.GroupInviteAllowed(context.Background(), &userv1.GroupInviteAllowedRequest{UserId: tc.user.String(), InviterId: inviter.String()})
		if err != nil || resp.GetAllowed() != tc.allowed {
			t.Errorf("%s: allowed=%v err=%v", name, resp.GetAllowed(), err)
		}
	}
	if _, err := internal.GroupInviteAllowed(context.Background(), &userv1.GroupInviteAllowedRequest{UserId: open.String()}); grpcxCode(err) != codes.InvalidArgument {
		t.Errorf("missing inviter: %v", err)
	}
	if _, err := internal.GroupInviteAllowed(context.Background(), &userv1.GroupInviteAllowedRequest{UserId: uuid.NewString(), InviterId: inviter.String()}); grpcxCode(err) != codes.NotFound {
		t.Errorf("unknown user: %v", err)
	}
}

func TestMessagingAllowedUserExistsPresenceVisible(t *testing.T) {
	d := newDeps()
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	d.blocks.blocked[[2]uuid.UUID{a, b}] = true
	d.profiles.byID[a] = models.Profile{UserID: a}
	d.settings.show = map[uuid.UUID]bool{a: true, b: false}
	_, internal := start(t, d)
	ctx := context.Background()

	r, err := internal.MessagingAllowed(ctx, &userv1.MessagingAllowedRequest{SenderId: b.String(), RecipientId: a.String()})
	if err != nil || r.GetAllowed() || r.GetBlockedBySender() || !r.GetBlockedByRecipient() {
		t.Fatalf("recipient blocked sender: %v %v", r, err)
	}
	if r, _ = internal.MessagingAllowed(ctx, &userv1.MessagingAllowedRequest{SenderId: a.String(), RecipientId: c.String()}); !r.GetAllowed() {
		t.Fatal("no block must be allowed")
	}

	if e, _ := internal.UserExists(ctx, &userv1.UserExistsRequest{UserId: a.String()}); !e.GetExists() {
		t.Fatal("known user")
	}
	if e, _ := internal.UserExists(ctx, &userv1.UserExistsRequest{UserId: c.String()}); e.GetExists() {
		t.Fatal("unknown user")
	}

	v, err := internal.PresenceVisible(ctx, &userv1.PresenceVisibleRequest{UserIds: []string{a.String(), b.String(), c.String()}})
	if err != nil || len(v.GetVisible()) != 2 || !v.GetVisible()[a.String()] || v.GetVisible()[b.String()] {
		t.Fatalf("presence visible: %v %v", v, err)
	}
}
