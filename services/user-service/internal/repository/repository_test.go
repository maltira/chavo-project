package repository_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/testutil"
)

func createProfile(t *testing.T, pool *pgxpool.Pool, username string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	err := repository.NewProfileRepository(pool).Create(context.Background(),
		&models.Profile{UserID: id, Username: username, DisplayName: "User " + username},
		&models.Settings{UserID: id})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestProfiles(t *testing.T) {
	pool := testutil.Pool(t, "user_repository_test")
	repo := repository.NewProfileRepository(pool)
	ctx := context.Background()

	id := createProfile(t, pool, "alice")
	createProfile(t, pool, "alicia")
	createProfile(t, pool, "bob")

	p, err := repo.FindByID(ctx, id)
	if err != nil || p.Username != "alice" || p.Bio != nil || p.LastSeenAt.IsZero() {
		t.Fatalf("find: %+v %v", p, err)
	}
	if _, err := repo.FindByID(ctx, uuid.New()); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}

	// Повторный профиль того же пользователя и занятый username различаются.
	err = repo.Create(ctx, &models.Profile{UserID: id, Username: "other", DisplayName: "x"}, &models.Settings{UserID: id})
	if !errors.Is(err, apperror.ErrProfileAlreadyExists) {
		t.Fatalf("duplicate profile: %v", err)
	}
	nid := uuid.New()
	err = repo.Create(ctx, &models.Profile{UserID: nid, Username: "alice", DisplayName: "x"}, &models.Settings{UserID: nid})
	if !errors.Is(err, apperror.ErrUsernameExists) {
		t.Fatalf("duplicate username: %v", err)
	}
	// Ошибка вставки профиля не оставляет настроек.
	if _, err := repository.NewSettingsRepository(pool).GetSettings(ctx, nid); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("settings of failed profile: %v", err)
	}
	if ok, _ := repo.UsernameExists(ctx, "alice"); !ok {
		t.Fatal("username exists")
	}
	if ok, _ := repo.UsernameExists(ctx, "nobody"); ok {
		t.Fatal("username does not exist")
	}

	found, err := repo.GetAllBySearch(ctx, "ali", 10, 0)
	if err != nil || len(found) != 2 || found[0].Username != "alice" || found[1].Username != "alicia" {
		t.Fatalf("search: %+v %v", found, err)
	}
	if page, _ := repo.GetAllBySearch(ctx, "ali", 1, 1); len(page) != 1 || page[0].Username != "alicia" {
		t.Fatalf("search page: %+v", page)
	}

	if err := repo.Update(ctx, id, map[string]any{"display_name": "Alice A.", "bio": "hi"}); err != nil {
		t.Fatal(err)
	}
	if p, _ := repo.FindByID(ctx, id); p.DisplayName != "Alice A." || p.Bio == nil || *p.Bio != "hi" {
		t.Fatalf("updated: %+v", p)
	}
	if err := repo.Update(ctx, id, map[string]any{"username": "bob"}); !errors.Is(err, apperror.ErrUsernameExists) {
		t.Fatalf("rename to taken: %v", err)
	}
	if err := repo.Update(ctx, id, map[string]any{"password": "x"}); err == nil {
		t.Fatal("unknown column accepted")
	}
	if err := repo.Update(ctx, uuid.New(), map[string]any{"bio": "x"}); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("update unknown: %v", err)
	}
	// Формат username проверяет БД.
	if err := repo.Update(ctx, id, map[string]any{"username": "bad name!"}); err == nil {
		t.Fatal("invalid username accepted")
	}

	// аватарка ставится путём и убирается nil - в БД NULL, а не пустая строка
	avatar := "/media/avatars/" + strings.Repeat("a", 32) + ".jpg"
	if err := repo.Update(ctx, id, map[string]any{"avatar_url": &avatar}); err != nil {
		t.Fatal(err)
	}
	if p, _ := repo.FindByID(ctx, id); p.AvatarURL == nil || *p.AvatarURL != avatar {
		t.Fatalf("avatar set: %+v", p.AvatarURL)
	}
	var none *string
	if err := repo.Update(ctx, id, map[string]any{"avatar_url": none}); err != nil {
		t.Fatal(err)
	}
	if p, _ := repo.FindByID(ctx, id); p.AvatarURL != nil {
		t.Fatalf("avatar must be NULL after clearing, got %q", *p.AvatarURL)
	}
}

func TestLastSeen(t *testing.T) {
	pool := testutil.Pool(t, "user_repository_test")
	repo := repository.NewProfileRepository(pool)
	ctx := context.Background()
	a, b := createProfile(t, pool, "seen_a"), createProfile(t, pool, "seen_b")

	at := time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond)
	if err := repo.UpdateLastSeenAt(ctx, a, at); err != nil {
		t.Fatal(err)
	}
	// Повтор и запоздавшее событие не откатывают время назад.
	if err := repo.UpdateLastSeenAt(ctx, a, at.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	seen, err := repo.LastSeenAt(ctx, []uuid.UUID{a, b, uuid.New()})
	if err != nil || len(seen) != 2 || !seen[a].Equal(at) || seen[b].IsZero() {
		t.Fatalf("last seen: %v %v", seen, err)
	}
	if seen, err := repo.LastSeenAt(ctx, nil); err != nil || len(seen) != 0 {
		t.Fatalf("empty: %v %v", seen, err)
	}
}

func TestSettings(t *testing.T) {
	pool := testutil.Pool(t, "user_repository_test")
	repo := repository.NewSettingsRepository(pool)
	ctx := context.Background()
	a, b := createProfile(t, pool, "set_a"), createProfile(t, pool, "set_b")

	s, err := repo.GetSettings(ctx, a)
	if err != nil || !s.AllowGroupInvites || !s.ShowOnlineStatus {
		t.Fatalf("defaults: %+v %v", s, err)
	}
	if err := repo.UpdateSettings(ctx, a, map[string]any{"show_online_status": false}); err != nil {
		t.Fatal(err)
	}
	if s, _ := repo.GetSettings(ctx, a); s.ShowOnlineStatus || !s.AllowGroupInvites {
		t.Fatalf("updated: %+v", s)
	}
	if err := repo.UpdateSettings(ctx, a, map[string]any{"theme": "dark"}); err == nil {
		t.Fatal("unknown column accepted")
	}
	if err := repo.UpdateSettings(ctx, uuid.New(), map[string]any{"show_online_status": true}); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("update unknown: %v", err)
	}

	visible, err := repo.ShowOnlineStatus(ctx, []uuid.UUID{a, b, uuid.New()})
	if err != nil || len(visible) != 2 || visible[a] || !visible[b] {
		t.Fatalf("visibility: %v %v", visible, err)
	}
}

func TestBlocks(t *testing.T) {
	pool := testutil.Pool(t, "user_repository_test")
	repo := repository.NewBlockRepository(pool)
	ctx := context.Background()
	a, b, c := createProfile(t, pool, "blk_a"), createProfile(t, pool, "blk_b"), createProfile(t, pool, "blk_c")

	for _, target := range []uuid.UUID{b, c, b} { // повтор не ошибка
		if err := repo.BlockUser(ctx, a, target); err != nil {
			t.Fatal(err)
		}
	}
	if ok, _ := repo.CheckBlock(ctx, a, b); !ok {
		t.Fatal("a blocks b")
	}
	if ok, _ := repo.CheckBlock(ctx, b, a); ok {
		t.Fatal("block is one-directional")
	}
	byA, byB, err := repo.CheckBlockBidirectional(ctx, b, a)
	if err != nil || byA || !byB {
		t.Fatalf("bidirectional: %v %v %v", byA, byB, err)
	}

	list, err := repo.GetBlockedUsers(ctx, a, 10, 0)
	if err != nil || len(list) != 2 || list[0].Username == "" {
		t.Fatalf("blocked list: %+v %v", list, err)
	}

	by, err := repo.BlockedBy(ctx, b, []uuid.UUID{a, c})
	if err != nil || len(by) != 1 || !by[a] {
		t.Fatalf("blocked by: %v %v", by, err)
	}
	if by, _ := repo.BlockedBy(ctx, b, nil); len(by) != 0 {
		t.Fatal("empty owners")
	}
	ids, err := repo.BlockedIDs(ctx, a)
	if err != nil || len(ids) != 2 {
		t.Fatalf("blocked ids: %v %v", ids, err)
	}

	if err := repo.UnblockUser(ctx, a, b); err != nil {
		t.Fatal(err)
	}
	if err := repo.UnblockUser(ctx, a, b); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("unblock twice: %v", err)
	}
	if list, _ := repo.GetBlockedUsers(ctx, a, 10, 0); len(list) != 1 || list[0].BlockedUserID != c {
		t.Fatalf("after unblock: %+v", list)
	}
}

func TestStatuses(t *testing.T) {
	pool := testutil.Pool(t, "user_repository_test")
	profiles := repository.NewProfileRepository(pool)
	statuses := repository.NewStatusRepository(pool)
	ctx := context.Background()
	id := createProfile(t, pool, "st_a")
	other := createProfile(t, pool, "st_b")

	if p, _ := profiles.FindByID(ctx, id); p.Status != nil {
		t.Fatal("new profile has a status")
	}
	exp := time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond)
	if err := statuses.Set(ctx, id, models.Status{Icon: "shine", Text: "Отдыхаю", Background: "#34D399", Foreground: "#064E3B", ExpiresAt: &exp}); err != nil {
		t.Fatal(err)
	}
	p, err := profiles.FindByID(ctx, id)
	if err != nil || p.Status == nil || p.Status.Icon != "shine" || p.Status.Text != "Отдыхаю" || p.Status.Background != "#34D399" || p.Status.Foreground != "#064E3B" || !p.Status.ExpiresAt.Equal(exp) {
		t.Fatalf("status: %+v %v", p, err)
	}

	// Установка заменяет статус целиком.
	if err := statuses.Set(ctx, id, models.Status{Icon: "moon", Text: "Ночь", Background: "#312E81", Foreground: "#E0E7FF"}); err != nil {
		t.Fatal(err)
	}
	if p, _ := profiles.FindByID(ctx, id); p.Status.Icon != "moon" || p.Status.Background != "#312E81" || p.Status.ExpiresAt != nil {
		t.Fatalf("replace: %+v", p.Status)
	}

	// Истёкший статус не читается ни в профиле, ни в поиске.
	past := time.Now().Add(-time.Minute)
	if err := statuses.Set(ctx, other, models.Status{Icon: "heart", Text: "был", Background: "#F43F5E", Foreground: "#FFFFFF", ExpiresAt: &past}); err != nil {
		t.Fatal(err)
	}
	if p, _ := profiles.FindByID(ctx, other); p.Status != nil {
		t.Fatalf("expired status read: %+v", p.Status)
	}
	found, err := profiles.GetAllBySearch(ctx, "st_", 10, 0)
	if err != nil || len(found) != 2 {
		t.Fatalf("search: %v %v", found, err)
	}
	for _, f := range found {
		if (f.UserID == id) != (f.Status != nil) {
			t.Fatalf("search status for %s: %+v", f.Username, f.Status)
		}
	}

	if removed, err := statuses.Clear(ctx, id); err != nil || !removed {
		t.Fatalf("clear: %v %v", removed, err)
	}
	if removed, _ := statuses.Clear(ctx, id); removed {
		t.Fatal("second clear reported a removal")
	}

	// Статус удаляется вместе с профилем.
	if _, err := pool.Exec(ctx, `DELETE FROM profiles WHERE user_id = $1`, other); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM user_statuses WHERE user_id = $1`, other).Scan(&n); err != nil || n != 0 {
		t.Fatalf("cascade: %d %v", n, err)
	}
}

func TestAvatarShapes(t *testing.T) {
	pool := testutil.Pool(t, "user_repository_test")
	profiles := repository.NewProfileRepository(pool)
	shapes := repository.NewAvatarShapeRepository(pool)
	ctx := context.Background()
	id := createProfile(t, pool, "sh_a")
	other := createProfile(t, pool, "sh_b")

	if p, _ := profiles.FindByID(ctx, id); p.AvatarShape != nil {
		t.Fatal("new profile has a shape")
	}
	if err := shapes.Set(ctx, id, models.AvatarShape{Shape: "4-leaf-clover", Color: "#34D399"}); err != nil {
		t.Fatal(err)
	}
	p, err := profiles.FindByID(ctx, id)
	if err != nil || p.AvatarShape == nil || p.AvatarShape.Shape != "4-leaf-clover" || p.AvatarShape.Color != "#34D399" {
		t.Fatalf("shape: %+v %v", p, err)
	}

	// установка заменяет фигуру целиком
	if err := shapes.Set(ctx, id, models.AvatarShape{Shape: "bun", Color: "#000000"}); err != nil {
		t.Fatal(err)
	}
	if p, _ := profiles.FindByID(ctx, id); p.AvatarShape.Shape != "bun" || p.AvatarShape.Color != "#000000" {
		t.Fatalf("replace: %+v", p.AvatarShape)
	}

	found, err := profiles.GetAllBySearch(ctx, "sh_", 10, 0)
	if err != nil || len(found) != 2 {
		t.Fatalf("search: %v %v", found, err)
	}
	for _, f := range found {
		if (f.UserID == id) != (f.AvatarShape != nil) {
			t.Fatalf("search shape for %s: %+v", f.Username, f.AvatarShape)
		}
	}

	if err := shapes.Clear(ctx, id); err != nil {
		t.Fatal(err)
	}
	if p, _ := profiles.FindByID(ctx, id); p.AvatarShape != nil {
		t.Fatalf("not cleared: %+v", p.AvatarShape)
	}
	if err := shapes.Clear(ctx, id); err != nil {
		t.Fatalf("second clear: %v", err)
	}

	// фигура удаляется вместе с профилем
	if err := shapes.Set(ctx, other, models.AvatarShape{Shape: "circle", Color: "#FFFFFF"}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM profiles WHERE user_id = $1`, other); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM user_avatar_shapes WHERE user_id = $1`, other).Scan(&n); err != nil || n != 0 {
		t.Fatalf("cascade: %d %v", n, err)
	}
}
