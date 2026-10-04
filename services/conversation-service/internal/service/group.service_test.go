package service_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
)

func createGroup(t *testing.T, e *env, creator uuid.UUID, visibility string, members ...uuid.UUID) *service.GroupResult {
	t.Helper()
	res, err := e.groups.Create(context.Background(), creator, service.CreateGroupInput{
		Name: "Группа", Visibility: visibility, MemberIDs: members,
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	return res
}

func memberRole(t *testing.T, e *env, conv, user uuid.UUID) string {
	t.Helper()
	var role string
	err := e.pool.QueryRow(context.Background(),
		`SELECT member_role FROM conversation_members WHERE conversation_id=$1 AND user_id=$2`, conv, user).Scan(&role)
	if err != nil {
		return ""
	}
	return role
}

func events(t *testing.T, e *env, typ string) int {
	t.Helper()
	return count(t, e.pool, `SELECT COUNT(*) FROM outbox_events WHERE payload->>'event_type' = $1`, typ)
}

func TestCreateGroup(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, m1, m2 := uuid.New(), uuid.New(), uuid.New()

	// private: токен возвращается один раз, в БД только его SHA-256.
	res := createGroup(t, e, admin, "", m1, m2, m1, admin)
	if res.InviteToken == nil || res.Conversation.Visibility != "private" {
		t.Fatalf("private group must have invite token: %+v", res)
	}
	var hash string
	if err := e.pool.QueryRow(ctx, `SELECT invite_token_hash FROM conversations WHERE id=$1`, res.Conversation.ID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(*res.InviteToken))
	if hash != hex.EncodeToString(sum[:]) || hash == *res.InviteToken {
		t.Fatal("stored hash must be sha256 of the token")
	}
	if memberRole(t, e, res.Conversation.ID, admin) != "admin" || memberRole(t, e, res.Conversation.ID, m1) != "member" {
		t.Fatal("wrong roles")
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM conversation_members WHERE conversation_id=$1`, res.Conversation.ID); n != 3 {
		t.Fatalf("members = %d, want 3 (duplicates and self removed)", n)
	}

	// public: токена нет.
	pub := createGroup(t, e, admin, "public")
	if pub.InviteToken != nil {
		t.Fatal("public group must not have invite token")
	}

	// Невалидные участники — ничего не создаётся.
	before := count(t, e.pool, `SELECT COUNT(*) FROM conversations`)
	e.users.missing[m2] = true
	if _, err := e.groups.Create(ctx, admin, service.CreateGroupInput{Name: "x", MemberIDs: []uuid.UUID{m2}}); !errors.Is(err, apperror.ErrUserNotFound) {
		t.Fatalf("missing user: %v", err)
	}
	delete(e.users.missing, m2)
	e.users.noInvites[m1] = true
	if _, err := e.groups.Create(ctx, admin, service.CreateGroupInput{Name: "x", MemberIDs: []uuid.UUID{m1}}); !errors.Is(err, apperror.ErrInviteNotAllowed) {
		t.Fatalf("invites forbidden: %v", err)
	}
	for name, in := range map[string]service.CreateGroupInput{
		"blank name":     {Name: "  "},
		"bad visibility": {Name: "x", Visibility: "secret"},
	} {
		if _, err := e.groups.Create(ctx, admin, in); !errors.Is(err, apperror.ErrIncorrectData) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if after := count(t, e.pool, `SELECT COUNT(*) FROM conversations`); after != before {
		t.Fatalf("failed creates left %d conversations", after-before)
	}
	if events(t, e, "conversation.created") != 2 {
		t.Fatalf("conversation.created events = %d", events(t, e, "conversation.created"))
	}
}

func TestUpdateGroup(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, member := uuid.New(), uuid.New()
	g := createGroup(t, e, admin, "private", member)
	id := g.Conversation.ID
	str := func(s string) *string { return &s }

	if _, err := e.groups.Update(ctx, member, id, service.UpdateGroupInput{Name: str("hack")}); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("member update: %v", err)
	}
	if _, err := e.groups.Update(ctx, uuid.New(), id, service.UpdateGroupInput{Name: str("hack")}); !errors.Is(err, apperror.ErrNotMember) {
		t.Fatalf("outsider update: %v", err)
	}
	if _, err := e.groups.Update(ctx, admin, id, service.UpdateGroupInput{}); !errors.Is(err, apperror.ErrIncorrectData) {
		t.Fatalf("empty update: %v", err)
	}
	if _, err := e.groups.Update(ctx, admin, id, service.UpdateGroupInput{Name: str(" ")}); !errors.Is(err, apperror.ErrIncorrectData) {
		t.Fatalf("blank name: %v", err)
	}

	res, err := e.groups.Update(ctx, admin, id, service.UpdateGroupInput{Name: str("  Новое имя "), Description: str("описание")})
	if err != nil || *res.Conversation.Name != "Новое имя" || *res.Conversation.Description != "описание" || res.InviteToken != nil {
		t.Fatalf("rename: %+v %v", res, err)
	}
	res, err = e.groups.Update(ctx, admin, id, service.UpdateGroupInput{Description: str("")})
	if err != nil || res.Conversation.Description != nil {
		t.Fatalf("clear description: %+v %v", res, err)
	}

	// private -> public: ссылка перестаёт действовать; public -> private: новый токен.
	res, err = e.groups.Update(ctx, admin, id, service.UpdateGroupInput{Visibility: str("public")})
	if err != nil || res.Conversation.Visibility != "public" || res.InviteToken != nil {
		t.Fatalf("to public: %+v %v", res, err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM conversations WHERE id=$1 AND invite_token_hash IS NULL`, id); n != 1 {
		t.Fatal("invite hash must be cleared for public group")
	}
	res, err = e.groups.Update(ctx, admin, id, service.UpdateGroupInput{Visibility: str("private")})
	if err != nil || res.InviteToken == nil {
		t.Fatalf("to private: %+v %v", res, err)
	}
	before := events(t, e, "conversation.updated")
	if _, err = e.groups.Update(ctx, admin, id, service.UpdateGroupInput{Visibility: str("private")}); err != nil {
		t.Fatal(err)
	}
	if events(t, e, "conversation.updated") != before {
		t.Fatal("no-op update must not publish an event")
	}

	// direct не имеет настроек.
	a, b := uuid.New(), uuid.New()
	m, _ := e.msgs.Send(ctx, a, service.SendMessageInput{RecipientID: &b, Content: "hi"})
	if _, err = e.groups.Update(ctx, a, m.ConversationID, service.UpdateGroupInput{Name: str("x")}); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("update direct: %v", err)
	}
}

func TestAddMembersAndJoin(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, member, newbie, locked := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	priv := createGroup(t, e, admin, "private", member).Conversation.ID
	pub := createGroup(t, e, admin, "public", member).Conversation.ID

	if err := e.groups.AddMembers(ctx, member, priv, []uuid.UUID{newbie}); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("member adds: %v", err)
	}
	e.users.noInvites[locked] = true
	if err := e.groups.AddMembers(ctx, admin, priv, []uuid.UUID{newbie, locked}); !errors.Is(err, apperror.ErrInviteNotAllowed) {
		t.Fatalf("locked: %v", err)
	}
	if memberRole(t, e, priv, newbie) != "" {
		t.Fatal("failed batch must not add anyone")
	}
	if err := e.groups.AddMembers(ctx, admin, priv, []uuid.UUID{newbie}); err != nil {
		t.Fatal(err)
	}
	if err := e.groups.AddMembers(ctx, admin, priv, []uuid.UUID{newbie}); !errors.Is(err, apperror.ErrAlreadyMember) {
		t.Fatalf("duplicate add: %v", err)
	}
	if err := e.groups.AddMembers(ctx, admin, priv, nil); !errors.Is(err, apperror.ErrIncorrectData) {
		t.Fatalf("empty add: %v", err)
	}

	// Join: public — можно, private — нет, повтор — 409.
	if err := e.groups.Join(ctx, newbie, pub); err != nil {
		t.Fatal(err)
	}
	if err := e.groups.Join(ctx, newbie, pub); !errors.Is(err, apperror.ErrAlreadyMember) {
		t.Fatalf("repeat join: %v", err)
	}
	if err := e.groups.Join(ctx, uuid.New(), priv); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("join private: %v", err)
	}
	if err := e.groups.Join(ctx, newbie, uuid.New()); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("join unknown: %v", err)
	}
	if events(t, e, "conversation.member.added") != 2 {
		t.Fatalf("member.added events = %d, want 2", events(t, e, "conversation.member.added"))
	}

	members, err := e.groups.ListMembers(ctx, member, priv, 10, 0)
	if err != nil || len(members) != 3 {
		t.Fatalf("members: %d %v", len(members), err)
	}
	if _, err = e.groups.ListMembers(ctx, uuid.New(), priv, 10, 0); !errors.Is(err, apperror.ErrNotMember) {
		t.Fatalf("outsider list: %v", err)
	}
}

func TestRolesAndLastAdminInvariant(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a1, a2, member := uuid.New(), uuid.New(), uuid.New()
	id := createGroup(t, e, a1, "private", a2, member).Conversation.ID

	if err := e.groups.SetRole(ctx, a1, id, a1, "member"); !errors.Is(err, apperror.ErrLastAdmin) {
		t.Fatalf("demote only admin: %v", err)
	}
	if err := e.groups.SetRole(ctx, member, id, member, "admin"); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("self-promote: %v", err)
	}
	if err := e.groups.SetRole(ctx, a1, id, a2, "owner"); !errors.Is(err, apperror.ErrIncorrectData) {
		t.Fatalf("bad role: %v", err)
	}
	if err := e.groups.SetRole(ctx, a1, id, uuid.New(), "admin"); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("unknown target: %v", err)
	}
	if err := e.groups.SetRole(ctx, a1, id, a2, "admin"); err != nil {
		t.Fatal(err)
	}
	if err := e.groups.SetRole(ctx, a1, id, a2, "admin"); err != nil {
		t.Fatalf("idempotent promote: %v", err)
	}

	// Два admin одновременно разжалуют друг друга: ровно один успех, админ остаётся.
	for i := 0; i < 20; i++ {
		_, _ = e.pool.Exec(ctx, `UPDATE conversation_members SET member_role='admin' WHERE conversation_id=$1 AND user_id IN ($2,$3)`, id, a1, a2)
		var wg sync.WaitGroup
		results := make(chan error, 2)
		for _, p := range [][2]uuid.UUID{{a1, a2}, {a2, a1}} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results <- e.groups.SetRole(ctx, p[0], id, p[1], "member")
			}()
		}
		wg.Wait()
		close(results)
		ok, last, other := 0, 0, 0
		for err := range results {
			switch {
			case err == nil:
				ok++
			case errors.Is(err, apperror.ErrLastAdmin), errors.Is(err, apperror.ErrForbidden):
				last++ // второй уже не admin или остался единственным
			default:
				other++
			}
		}
		if ok != 1 || other != 0 {
			t.Fatalf("iter %d: ok=%d last=%d other=%d", i, ok, last, other)
		}
		if n := count(t, e.pool, `SELECT COUNT(*) FROM conversation_members WHERE conversation_id=$1 AND member_role='admin'`, id); n < 1 {
			t.Fatalf("iter %d: group left without admin", i)
		}
	}
}

func TestRemoveMemberLeaveAndDelete(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, m1, m2 := uuid.New(), uuid.New(), uuid.New()
	id := createGroup(t, e, admin, "private", m1, m2).Conversation.ID

	if err := e.groups.RemoveMember(ctx, m1, id, m2, false); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("member kicks: %v", err)
	}
	if err := e.groups.RemoveMember(ctx, admin, id, uuid.New(), false); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kick non-member: %v", err)
	}
	if err := e.groups.RemoveMember(ctx, admin, id, m2, false); err != nil {
		t.Fatal(err)
	}
	if memberRole(t, e, id, m2) != "" {
		t.Fatal("kicked member still present")
	}
	if err := e.groups.RemoveMember(ctx, admin, id, admin, false); !errors.Is(err, apperror.ErrLastAdmin) {
		t.Fatalf("last admin leaves: %v", err)
	}
	if err := e.groups.RemoveMember(ctx, admin, id, m1, false); err != nil {
		t.Fatal(err)
	}
	if events(t, e, "conversation.member.removed") != 2 {
		t.Fatalf("member.removed events = %d", events(t, e, "conversation.member.removed"))
	}

	// DELETE: участник выходит, admin удаляет группу вместе с сообщениями.
	id2 := createGroup(t, e, admin, "private", m1).Conversation.ID
	if _, err := e.msgs.Send(ctx, m1, service.SendMessageInput{ConversationID: &id2, Content: "hi"}); err != nil {
		t.Fatal(err)
	}
	if err := e.groups.Delete(ctx, m1, id2); err != nil {
		t.Fatal(err)
	}
	if memberRole(t, e, id2, m1) != "" || count(t, e.pool, `SELECT COUNT(*) FROM conversations WHERE id=$1`, id2) != 1 {
		t.Fatal("member DELETE must only leave the group")
	}
	if err := e.groups.Delete(ctx, m1, id2); !errors.Is(err, apperror.ErrNotMember) {
		t.Fatalf("delete after leave: %v", err)
	}
	if err := e.groups.Delete(ctx, admin, id2); err != nil {
		t.Fatal(err)
	}
	if count(t, e.pool, `SELECT COUNT(*) FROM conversations WHERE id=$1`, id2) != 0 ||
		count(t, e.pool, `SELECT COUNT(*) FROM messages WHERE conversation_id=$1`, id2) != 0 {
		t.Fatal("group and its messages must be deleted")
	}
	if events(t, e, "conversation.deleted") != 1 {
		t.Fatalf("conversation.deleted events = %d", events(t, e, "conversation.deleted"))
	}

	// direct нельзя ни удалить, ни покинуть.
	a, b := uuid.New(), uuid.New()
	m, _ := e.msgs.Send(ctx, a, service.SendMessageInput{RecipientID: &b, Content: "hi"})
	if err := e.groups.Delete(ctx, a, m.ConversationID); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("delete direct: %v", err)
	}
	if err := e.groups.RemoveMember(ctx, a, m.ConversationID, a, false); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("leave direct: %v", err)
	}
}

func TestGroupInviteRespectsBlocks(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, blockedByAdmin, blockedAdmin, ok := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	e.users.blockedBySender[[2]uuid.UUID{admin, blockedByAdmin}] = true // админ заблокировал
	e.users.blockedBySender[[2]uuid.UUID{blockedAdmin, admin}] = true   // админа заблокировали

	for name, target := range map[string]uuid.UUID{"blocked by admin": blockedByAdmin, "admin is blocked": blockedAdmin} {
		_, err := e.groups.Create(ctx, admin, service.CreateGroupInput{Name: "g", MemberIDs: []uuid.UUID{target}})
		if !errors.Is(err, apperror.ErrInviteNotAllowed) {
			t.Errorf("create, %s: %v", name, err)
		}
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM conversations`); n != 0 {
		t.Fatalf("rejected create left %d conversations", n)
	}

	conv := createGroup(t, e, admin, "", ok).Conversation.ID
	for name, target := range map[string]uuid.UUID{"blocked by admin": blockedByAdmin, "admin is blocked": blockedAdmin} {
		if err := e.groups.AddMembers(ctx, admin, conv, []uuid.UUID{target}); !errors.Is(err, apperror.ErrInviteNotAllowed) {
			t.Errorf("add, %s: %v", name, err)
		}
	}
	if memberRole(t, e, conv, blockedByAdmin) != "" || memberRole(t, e, conv, blockedAdmin) != "" {
		t.Fatal("blocked users must not become members")
	}
	// Блокировка касается только пары: остальным админ приглашать по-прежнему может.
	other := uuid.New()
	if err := e.groups.AddMembers(ctx, admin, conv, []uuid.UUID{other}); err != nil {
		t.Fatalf("add unrelated user: %v", err)
	}
}

func TestAvatarURLValidation(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin := uuid.New()
	bad := []string{"javascript:alert(1)", "ftp://host/a.png", "http:///no-host", "not a url", "//host/a.png"}

	for _, u := range bad {
		u := u
		if _, err := e.groups.Create(ctx, admin, service.CreateGroupInput{Name: "g", AvatarURL: &u}); !errors.Is(err, apperror.ErrIncorrectData) {
			t.Errorf("create with %q: %v", u, err)
		}
	}
	good := "https://cdn.example.com/a.png"
	res, err := e.groups.Create(ctx, admin, service.CreateGroupInput{Name: "g", AvatarURL: &good})
	if err != nil || res.Conversation.AvatarURL == nil || *res.Conversation.AvatarURL != good {
		t.Fatalf("create with valid url: %+v %v", res, err)
	}
	id := res.Conversation.ID
	for _, u := range bad {
		u := u
		if _, err = e.groups.Update(ctx, admin, id, service.UpdateGroupInput{AvatarURL: &u}); !errors.Is(err, apperror.ErrIncorrectData) {
			t.Errorf("update with %q: %v", u, err)
		}
	}
	empty := "  "
	res, err = e.groups.Update(ctx, admin, id, service.UpdateGroupInput{AvatarURL: &empty})
	if err != nil || res.Conversation.AvatarURL != nil {
		t.Fatalf("blank url must clear the avatar: %+v %v", res, err)
	}
}

func fillMembers(t *testing.T, e *env, conv uuid.UUID, n int) {
	t.Helper()
	if _, err := e.pool.Exec(context.Background(),
		`INSERT INTO conversation_members (conversation_id, user_id) SELECT $1, gen_random_uuid() FROM generate_series(1, $2)`, conv, n); err != nil {
		t.Fatal(err)
	}
}

func TestPublicGroupMemberLimit(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin := uuid.New()
	conv := createGroup(t, e, admin, "public").Conversation.ID

	fillMembers(t, e, conv, 4998) // вместе с admin 4999
	if err := e.groups.Join(ctx, uuid.New(), conv); err != nil {
		t.Fatalf("join up to the limit: %v", err)
	}
	if err := e.groups.Join(ctx, uuid.New(), conv); !errors.Is(err, apperror.ErrGroupFull) {
		t.Fatalf("join over the limit: %v", err)
	}
	if err := e.groups.AddMembers(ctx, admin, conv, []uuid.UUID{uuid.New()}); !errors.Is(err, apperror.ErrGroupFull) {
		t.Fatalf("add over the limit: %v", err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM conversation_members WHERE conversation_id = $1`, conv); n != 5000 {
		t.Fatalf("members = %d, want 5000", n)
	}
}

func TestAddMembersCannotCrossLimit(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin := uuid.New()
	conv := createGroup(t, e, admin, "").Conversation.ID

	fillMembers(t, e, conv, 4998) // 4999
	if err := e.groups.AddMembers(ctx, admin, conv, []uuid.UUID{uuid.New(), uuid.New()}); !errors.Is(err, apperror.ErrGroupFull) {
		t.Fatalf("batch crossing the limit: %v", err)
	}
	if n := count(t, e.pool, `SELECT COUNT(*) FROM conversation_members WHERE conversation_id = $1`, conv); n != 4999 {
		t.Fatalf("rejected batch must add nobody, members = %d", n)
	}
	if err := e.groups.AddMembers(ctx, admin, conv, []uuid.UUID{uuid.New()}); err != nil {
		t.Fatalf("exactly one slot left: %v", err)
	}
}
