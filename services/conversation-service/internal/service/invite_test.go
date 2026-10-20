package service_test

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
)

func str(s string) *string { return &s }

// testSlug - уникальный код ссылки открытой группы
func testSlug() string {
	return "g_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
}

var inviteCodeRe = regexp.MustCompile(`^[a-z0-9]{12}$`)

func TestPrivateInviteCode(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, member, guest := uuid.New(), uuid.New(), uuid.New()
	g := createGroup(t, e, admin, "private", member)
	id := g.Conversation.ID
	code := *g.InviteToken
	if !inviteCodeRe.MatchString(code) {
		t.Fatalf("invite code format: %q", code)
	}

	// в БД код только хэшем и шифротекстом
	var enc []byte
	if err := e.pool.QueryRow(ctx, `SELECT invite_token_enc FROM conversations WHERE id=$1`, id).Scan(&enc); err != nil {
		t.Fatal(err)
	}
	if len(enc) == 0 || bytes.Contains(enc, []byte(code)) {
		t.Fatal("invite code must be stored encrypted")
	}

	got, err := e.convs.Get(ctx, admin, id)
	if err != nil || got.InviteCode == nil || *got.InviteCode != code || got.MembersCount != 2 {
		t.Fatalf("admin view: %+v %v", got, err)
	}
	got, err = e.convs.Get(ctx, member, id)
	if err != nil || got.InviteCode != nil {
		t.Fatalf("member must not see the code: %+v %v", got, err)
	}
	list, err := e.convs.List(ctx, admin, 10, 0)
	if err != nil || len(list) != 1 || list[0].InviteCode != nil {
		t.Fatalf("list must not carry the code: %+v %v", list, err)
	}

	info, err := e.joins.InviteInfo(ctx, guest, strings.ToUpper(code))
	if err != nil || info.ConversationID != id || info.Visibility != "private" || info.Slug != nil {
		t.Fatalf("invite info by code: %+v %v", info, err)
	}

	fresh, err := e.joins.RegenerateInvite(ctx, admin, id)
	if err != nil || fresh == code || !inviteCodeRe.MatchString(fresh) {
		t.Fatalf("regenerate: %q %v", fresh, err)
	}
	if _, err = e.joins.InviteInfo(ctx, guest, code); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("old code must stop working: %v", err)
	}
	if got, _ = e.convs.Get(ctx, admin, id); got.InviteCode == nil || *got.InviteCode != fresh {
		t.Fatalf("admin must see the new code: %+v", got)
	}
	if _, err = e.joins.RequestJoin(ctx, guest, id, fresh); err != nil {
		t.Fatalf("request join by new code: %v", err)
	}
}

func TestPublicSlug(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, guest := uuid.New(), uuid.New()
	create := func(slug *string) (*service.GroupResult, error) {
		return e.groups.Create(ctx, admin, service.CreateGroupInput{Name: "Книжный клуб", Visibility: "public", Slug: slug})
	}

	if _, err := create(nil); !errors.Is(err, apperror.ErrSlugRequired) {
		t.Fatalf("no slug: %v", err)
	}
	for _, bad := range []string{"ab", "book-club", "книги", strings.Repeat("a", 33)} {
		if _, err := create(str(bad)); !errors.Is(err, apperror.ErrSlugInvalid) {
			t.Fatalf("slug %q: %v", bad, err)
		}
	}
	res, err := create(str(" Book_Club "))
	if err != nil || res.InviteToken != nil || res.Conversation.Slug == nil || *res.Conversation.Slug != "Book_Club" {
		t.Fatalf("create: %+v %v", res, err)
	}
	if _, err = create(str("book_club")); !errors.Is(err, apperror.ErrSlugTaken) {
		t.Fatalf("slug must be unique ignoring case: %v", err)
	}

	info, err := e.joins.InviteInfo(ctx, guest, "BOOK_CLUB")
	if err != nil || info.ConversationID != res.Conversation.ID || info.Visibility != "public" || *info.Slug != "Book_Club" {
		t.Fatalf("invite info by slug: %+v %v", info, err)
	}
	found, err := e.convs.SearchPublicGroups(ctx, guest, "ok_cl", 10, 0)
	if err != nil || len(found) != 1 || found[0].Slug != "Book_Club" {
		t.Fatalf("search by slug: %+v %v", found, err)
	}

	// код закрытой группы нельзя занять как slug: ссылки обоих видов живут в одном /chat/<код>
	private := createGroup(t, e, admin, "private")
	if _, err = create(str(strings.ToUpper(*private.InviteToken))); !errors.Is(err, apperror.ErrSlugTaken) {
		t.Fatalf("slug equal to a private code: %v", err)
	}
}

func TestSwitchVisibilityLinks(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, guest := uuid.New(), uuid.New()
	g := createGroup(t, e, admin, "private")
	id, code := g.Conversation.ID, *g.InviteToken

	if _, err := e.groups.Update(ctx, admin, id, service.UpdateGroupInput{Visibility: str("public")}); !errors.Is(err, apperror.ErrSlugRequired) {
		t.Fatalf("to public without slug: %v", err)
	}
	slug := testSlug()
	res, err := e.groups.Update(ctx, admin, id, service.UpdateGroupInput{Visibility: str("public"), Slug: str(slug)})
	if err != nil || *res.Conversation.Slug != slug || res.Conversation.InviteCodeEnc != nil {
		t.Fatalf("to public: %+v %v", res, err)
	}
	if _, err = e.joins.InviteInfo(ctx, guest, code); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("private code after switch: %v", err)
	}

	renamed := testSlug()
	if _, err = e.groups.Update(ctx, admin, id, service.UpdateGroupInput{Slug: str(renamed)}); err != nil {
		t.Fatal(err)
	}
	if _, err = e.joins.InviteInfo(ctx, guest, slug); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("old slug: %v", err)
	}
	if _, err = e.groups.Update(ctx, admin, id, service.UpdateGroupInput{Slug: str("bad slug")}); !errors.Is(err, apperror.ErrSlugInvalid) {
		t.Fatalf("invalid slug: %v", err)
	}

	res, err = e.groups.Update(ctx, admin, id, service.UpdateGroupInput{Visibility: str("private")})
	if err != nil || res.Conversation.Slug != nil || res.InviteToken == nil {
		t.Fatalf("to private: %+v %v", res, err)
	}
	if _, err = e.joins.InviteInfo(ctx, guest, renamed); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("slug after switch: %v", err)
	}
	if info, err := e.joins.InviteInfo(ctx, guest, *res.InviteToken); err != nil || info.ConversationID != id {
		t.Fatalf("new private code: %+v %v", info, err)
	}
}
