package repository_test

import (
	"context"
	"errors"
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

	if err := repo.Update(ctx, id, map[string]string{"display_name": "Alice A.", "bio": "hi"}); err != nil {
		t.Fatal(err)
	}
	if p, _ := repo.FindByID(ctx, id); p.DisplayName != "Alice A." || p.Bio == nil || *p.Bio != "hi" {
		t.Fatalf("updated: %+v", p)
	}
	if err := repo.Update(ctx, id, map[string]string{"username": "bob"}); !errors.Is(err, apperror.ErrUsernameExists) {
		t.Fatalf("rename to taken: %v", err)
	}
	if err := repo.Update(ctx, id, map[string]string{"password": "x"}); err == nil {
		t.Fatal("unknown column accepted")
	}
	if err := repo.Update(ctx, uuid.New(), map[string]string{"bio": "x"}); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("update unknown: %v", err)
	}
	// Формат username проверяет БД.
	if err := repo.Update(ctx, id, map[string]string{"username": "bad name!"}); err == nil {
		t.Fatal("invalid username accepted")
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
