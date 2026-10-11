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

func (f *fakeProfiles) LastSeen(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]time.Time, error) {
	res := map[uuid.UUID]time.Time{}
	for _, id := range ids {
		if p, ok := f.byID[id]; ok {
			res[id] = p.LastSeenAt
		}
	}
	return res, nil
}

type fakeBlocks struct {
	service.BlockService
	blocked map[[2]uuid.UUID]bool // {blocker, blocked}
}

func (f *fakeBlocks) GetBlockStatus(_ context.Context, my, target uuid.UUID) (bool, bool, error) {
	return f.blocked[[2]uuid.UUID{my, target}], f.blocked[[2]uuid.UUID{target, my}], nil
}

func (f *fakeBlocks) BlockedBy(_ context.Context, viewer uuid.UUID, owners []uuid.UUID) (map[uuid.UUID]bool, error) {
	res := map[uuid.UUID]bool{}
	for _, o := range owners {
		if f.blocked[[2]uuid.UUID{o, viewer}] {
			res[o] = true
		}
	}
	return res, nil
}

func (f *fakeBlocks) BlockedIDs(_ context.Context, owner uuid.UUID) ([]uuid.UUID, error) {
	res := []uuid.UUID{}
	for pair := range f.blocked {
		if pair[0] == owner {
			res = append(res, pair[1])
		}
	}
	return res, nil
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

// fakeStatusRepo пишет статус прямо в профиль fakeProfiles — так его видят GetProfile и поиск.
type fakeStatusRepo struct{ profiles *fakeProfiles }

func (r *fakeStatusRepo) Set(_ context.Context, id uuid.UUID, st models.Status) error {
	p := r.profiles.byID[id]
	p.Status = &st
	r.profiles.byID[id] = p
	return nil
}

func (r *fakeStatusRepo) Clear(_ context.Context, id uuid.UUID) (bool, error) {
	p := r.profiles.byID[id]
	had := p.Status != nil
	p.Status = nil
	r.profiles.byID[id] = p
	return had, nil
}

// fakeShapeRepo пишет фигуру прямо в профиль fakeProfiles, как fakeStatusRepo
type fakeShapeRepo struct{ profiles *fakeProfiles }

func (r *fakeShapeRepo) Set(_ context.Context, id uuid.UUID, sh models.AvatarShape) error {
	p := r.profiles.byID[id]
	p.AvatarShape = &sh
	r.profiles.byID[id] = p
	return nil
}

func (r *fakeShapeRepo) Clear(_ context.Context, id uuid.UUID) error {
	p := r.profiles.byID[id]
	p.AvatarShape = nil
	r.profiles.byID[id] = p
	return nil
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
	statuses := service.NewStatusService(&fakeStatusRepo{profiles: d.profiles})
	shapes := service.NewAvatarShapeService(&fakeShapeRepo{profiles: d.profiles})
	impl := grpcserver.New(d.profiles, d.blocks, d.settings, statuses, shapes, d.presence)
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
	if len(v.GetLastSeenAt()) != 0 {
		t.Fatal("last_seen_at returned without with_last_seen")
	}

	// last_seen_at — только для тех, кто показывает онлайн.
	seen := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	d.profiles.byID[b] = models.Profile{UserID: b, LastSeenAt: seen}
	d.profiles.byID[a] = models.Profile{UserID: a, LastSeenAt: seen}
	v, err = internal.PresenceVisible(ctx, &userv1.PresenceVisibleRequest{UserIds: []string{a.String(), b.String()}, WithLastSeen: true})
	if err != nil || len(v.GetLastSeenAt()) != 1 || !v.GetLastSeenAt()[a.String()].AsTime().Equal(seen) {
		t.Fatalf("last seen: %v %v", v, err)
	}
}

func TestBlockedViewerSeesNoAvatarOrPresence(t *testing.T) {
	d := newDeps()
	owner, blocked, other := uuid.New(), uuid.New(), uuid.New()
	avatar := "https://example.com/a.png"
	bio := "about me"
	seen := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, id := range []uuid.UUID{owner, blocked, other} {
		d.profiles.byID[id] = models.Profile{UserID: id, Username: "u", DisplayName: "U", Bio: &bio, AvatarURL: &avatar, LastSeenAt: seen}
		d.presence.online[id] = true
		d.settings.show[id] = true
	}
	d.blocks.blocked[[2]uuid.UUID{owner, blocked}] = true // owner заблокировал blocked
	users, internal := start(t, d)

	p, err := users.GetProfile(as(blocked), &userv1.GetProfileRequest{UserId: owner.String()})
	got := p.GetProfile()
	if err != nil || got.AvatarUrl != nil || got.GetOnline() || got.GetLastSeenAt() != nil {
		t.Fatalf("blocked viewer sees hidden fields: %v %v", got, err)
	}
	if got.GetUsername() != "u" || got.GetDisplayName() != "U" || got.GetBio() != bio {
		t.Fatalf("name, username and bio must stay visible: %v", got)
	}

	// Блокировка односторонняя: owner видит blocked полностью, посторонний — owner полностью.
	for _, tc := range []struct{ viewer, target uuid.UUID }{{owner, blocked}, {other, owner}} {
		p, err := users.GetProfile(as(tc.viewer), &userv1.GetProfileRequest{UserId: tc.target.String()})
		if err != nil || p.GetProfile().GetAvatarUrl() != avatar || !p.GetProfile().GetOnline() {
			t.Fatalf("%s → %s: %v %v", tc.viewer, tc.target, p, err)
		}
	}

	found, err := users.SearchProfiles(as(blocked), &userv1.SearchProfilesRequest{Query: "u"})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range found.GetProfiles() {
		if p.GetUserId() == owner.String() && (p.AvatarUrl != nil || p.GetOnline()) {
			t.Fatalf("search leaks owner to blocked viewer: %v", p)
		}
	}

	viewer := blocked.String()
	v, err := internal.PresenceVisible(context.Background(), &userv1.PresenceVisibleRequest{
		UserIds: []string{owner.String(), other.String()}, WithLastSeen: true, ViewerId: &viewer,
	})
	if err != nil || v.GetVisible()[owner.String()] || !v.GetVisible()[other.String()] || v.GetLastSeenAt()[owner.String()] != nil {
		t.Fatalf("presence for blocked viewer: %v %v", v, err)
	}
	ids, err := internal.BlockedIDs(context.Background(), &userv1.BlockedIDsRequest{UserId: owner.String()})
	if err != nil || len(ids.GetUserIds()) != 1 || ids.GetUserIds()[0] != blocked.String() {
		t.Fatalf("blocked ids: %v %v", ids, err)
	}
}

func TestStatusSetClearAndBlockHiding(t *testing.T) {
	d := newDeps()
	owner, blocked, other := uuid.New(), uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{owner, blocked, other} {
		d.profiles.byID[id] = models.Profile{UserID: id, Username: "u", DisplayName: "U"}
	}
	d.blocks.blocked[[2]uuid.UUID{owner, blocked}] = true
	users, _ := start(t, d)

	set, err := users.SetStatus(as(owner), &userv1.SetStatusRequest{
		Icon: "shine", Text: "  Сегодня отдыхаю ", Background: "#34d399", Foreground: "#064E3B", Duration: "4h",
	})
	if err != nil || set.GetStatus().GetIcon() != "shine" || set.GetStatus().GetText() != "Сегодня отдыхаю" ||
		set.GetStatus().GetBackground() != "#34D399" || set.GetStatus().GetForeground() != "#064E3B" || set.GetStatus().GetExpiresAt() == nil {
		t.Fatalf("set: %v %v", set, err)
	}

	p, _ := users.GetProfile(as(other), &userv1.GetProfileRequest{UserId: owner.String()})
	if p.GetProfile().GetStatus().GetText() != "Сегодня отдыхаю" {
		t.Fatalf("other must see status: %v", p)
	}
	p, _ = users.GetProfile(as(blocked), &userv1.GetProfileRequest{UserId: owner.String()})
	if p.GetProfile().GetStatus() != nil {
		t.Fatalf("blocked viewer sees status: %v", p)
	}
	me, _ := users.GetMe(as(owner), &userv1.GetMeRequest{})
	if me.GetProfile().GetStatus().GetIcon() != "shine" {
		t.Fatalf("own status: %v", me)
	}

	// Новый статус заменяет прежний целиком; без срока — expires_at не задан.
	set, err = users.SetStatus(as(owner), &userv1.SetStatusRequest{
		Icon: "moon", Text: "Ночь", Background: "#312E81", Foreground: "#E0E7FF", Duration: "never",
	})
	if err != nil || set.GetStatus().GetIcon() != "moon" || set.GetStatus().GetBackground() != "#312E81" || set.GetStatus().GetExpiresAt() != nil {
		t.Fatalf("replace: %v %v", set, err)
	}

	for _, req := range []*userv1.SetStatusRequest{
		{Icon: "harmony", Text: "x", Background: "#000000", Foreground: "#FFFFFF", Duration: "1h"},
		{Icon: "heart", Text: "x", Background: "#000000", Foreground: "#FFFFFF", Duration: "2h"},
		{Icon: "heart", Text: "", Background: "#000000", Foreground: "#FFFFFF", Duration: "1h"},
		{Icon: "heart", Text: "a\nb", Background: "#000000", Foreground: "#FFFFFF", Duration: "1h"},
		{Icon: "heart", Text: "x", Foreground: "#FFFFFF", Duration: "1h"},
		{Icon: "heart", Text: "x", Background: "#000000", Foreground: "white", Duration: "1h"},
	} {
		_, err := users.SetStatus(as(owner), req)
		if grpcxCode(err) != codes.InvalidArgument {
			t.Fatalf("%v: %v", req, err)
		}
	}
	if _, err := users.SetStatus(context.Background(), &userv1.SetStatusRequest{Icon: "heart", Text: "x", Background: "#000000", Foreground: "#FFFFFF", Duration: "1h"}); grpcxCode(err) != codes.Unauthenticated {
		t.Fatalf("anonymous set: %v", err)
	}

	if _, err := users.ClearStatus(as(owner), &userv1.ClearStatusRequest{}); err != nil {
		t.Fatal(err)
	}
	if me, _ := users.GetMe(as(owner), &userv1.GetMeRequest{}); me.GetProfile().GetStatus() != nil {
		t.Fatal("status not cleared")
	}
	// повторное удаление - не ошибка
	if _, err := users.ClearStatus(as(owner), &userv1.ClearStatusRequest{}); err != nil {
		t.Fatal(err)
	}

}

func TestAvatarShapeSetClearAndBlockHiding(t *testing.T) {
	d := newDeps()
	owner, blocked, other := uuid.New(), uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{owner, blocked, other} {
		d.profiles.byID[id] = models.Profile{UserID: id, Username: "u", DisplayName: "U"}
	}
	d.blocks.blocked[[2]uuid.UUID{owner, blocked}] = true
	users, _ := start(t, d)

	set, err := users.SetAvatarShape(as(owner), &userv1.SetAvatarShapeRequest{Shape: "4-leaf-clover", Color: "#34d399"})
	if err != nil || set.GetAvatarShape().GetShape() != "4-leaf-clover" || set.GetAvatarShape().GetColor() != "#34D399" {
		t.Fatalf("set: %v %v", set, err)
	}

	p, _ := users.GetProfile(as(other), &userv1.GetProfileRequest{UserId: owner.String()})
	if p.GetProfile().GetAvatarShape().GetShape() != "4-leaf-clover" {
		t.Fatalf("other must see shape: %v", p)
	}
	p, _ = users.GetProfile(as(blocked), &userv1.GetProfileRequest{UserId: owner.String()})
	if p.GetProfile().GetAvatarShape() != nil {
		t.Fatalf("blocked viewer sees shape: %v", p)
	}
	found, _ := users.SearchProfiles(as(blocked), &userv1.SearchProfilesRequest{Query: "u"})
	for _, f := range found.GetProfiles() {
		if f.GetUserId() == owner.String() && f.GetAvatarShape() != nil {
			t.Fatalf("blocked viewer sees shape in search: %v", f)
		}
	}

	// новая фигура заменяет прежнюю целиком
	if set, err = users.SetAvatarShape(as(owner), &userv1.SetAvatarShapeRequest{Shape: "bun", Color: "#000000"}); err != nil || set.GetAvatarShape().GetShape() != "bun" {
		t.Fatalf("replace: %v %v", set, err)
	}
	if me, _ := users.GetMe(as(owner), &userv1.GetMeRequest{}); me.GetProfile().GetAvatarShape().GetColor() != "#000000" {
		t.Fatalf("own shape: %v", me)
	}

	for _, req := range []*userv1.SetAvatarShapeRequest{
		{Shape: "Circle", Color: "#000000"},
		{Shape: "hexagon", Color: "#000000"},
		{Shape: "circle"},
		{Shape: "circle", Color: "black"},
	} {
		if _, err := users.SetAvatarShape(as(owner), req); grpcxCode(err) != codes.InvalidArgument {
			t.Fatalf("%v: %v", req, err)
		}
	}
	if _, err := users.SetAvatarShape(context.Background(), &userv1.SetAvatarShapeRequest{Shape: "circle", Color: "#000000"}); grpcxCode(err) != codes.Unauthenticated {
		t.Fatalf("anonymous set: %v", err)
	}

	// фигура, убранная из каталога после установки, читается как отсутствующая
	p0 := d.profiles.byID[other]
	p0.AvatarShape = &models.AvatarShape{Shape: "hexagon", Color: "#000000"}
	d.profiles.byID[other] = p0
	if p, _ := users.GetProfile(as(owner), &userv1.GetProfileRequest{UserId: other.String()}); p.GetProfile().GetAvatarShape() != nil {
		t.Fatalf("unknown shape read: %v", p)
	}

	if _, err := users.ClearAvatarShape(as(owner), &userv1.ClearAvatarShapeRequest{}); err != nil {
		t.Fatal(err)
	}
	if me, _ := users.GetMe(as(owner), &userv1.GetMeRequest{}); me.GetProfile().GetAvatarShape() != nil {
		t.Fatal("shape not cleared")
	}
	// повторное удаление - не ошибка
	if _, err := users.ClearAvatarShape(as(owner), &userv1.ClearAvatarShapeRequest{}); err != nil {
		t.Fatal(err)
	}
}
