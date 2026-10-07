package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maltira/chavo-project-backend/services/auth-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/testutil"
)

func ptr(s string) *string { return &s }

func newUser(t *testing.T, pool *pgxpool.Pool, email string) *models.User {
	t.Helper()
	u, err := repository.NewUserRepository(pool).CreateOrUpdateUnverified(context.Background(), email, "hash-1")
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func verify(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := repository.NewUserRepository(pool).SetVerifiedTx(ctx, tx, id); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestUsers(t *testing.T) {
	pool := testutil.Pool(t, "auth_repository_test")
	repo := repository.NewUserRepository(pool)
	ctx := context.Background()

	u := newUser(t, pool, "alice@example.com")
	if u.ID == uuid.Nil || u.EmailVerified || u.PasswordHash != "hash-1" {
		t.Fatalf("created %+v", u)
	}

	// Повторная регистрация до подтверждения меняет пароль той же записи.
	again, err := repo.CreateOrUpdateUnverified(ctx, "alice@example.com", "hash-2")
	if err != nil || again.ID != u.ID || again.PasswordHash != "hash-2" {
		t.Fatalf("re-register: %+v %v", again, err)
	}

	verify(t, pool, u.ID)
	if _, err := repo.CreateOrUpdateUnverified(ctx, "alice@example.com", "hash-3"); !errors.Is(err, apperror.ErrEmailExists) {
		t.Fatalf("re-register verified: %v", err)
	}

	got, err := repo.FindByEmail(ctx, "alice@example.com")
	if err != nil || got.ID != u.ID || !got.EmailVerified || got.PasswordHash != "hash-2" {
		t.Fatalf("find by email: %+v %v", got, err)
	}
	if _, err := repo.FindByEmail(ctx, "nobody@example.com"); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("unknown email: %v", err)
	}
	if _, err := repo.FindByID(ctx, uuid.New()); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("unknown id: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdatePasswordHashTx(ctx, tx, u.ID, "hash-new"); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdatePasswordHashTx(ctx, tx, uuid.New(), "x"); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("update unknown: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.FindByID(ctx, u.ID); got.PasswordHash != "hash-new" {
		t.Fatalf("password not updated: %q", got.PasswordHash)
	}

	// Ограничение формата email — в БД.
	if _, err := repo.CreateOrUpdateUnverified(ctx, "not-an-email", "h"); err == nil {
		t.Fatal("invalid email accepted")
	}
}

func TestRefreshTokens(t *testing.T) {
	pool := testutil.Pool(t, "auth_repository_test")
	repo := repository.NewTokenRepository(pool)
	ctx := context.Background()
	u := newUser(t, pool, "bob@example.com")
	other := newUser(t, pool, "carol@example.com")
	exp := time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond)

	// ip_address — INET: чтение должно возвращать адрес строкой (IPv4, IPv6 и NULL).
	sessions := []*models.RefreshToken{
		{UserID: u.ID, TokenHash: "h-v4", DeviceName: ptr("Chrome on Linux"), UserAgent: ptr("Mozilla/5.0"), IPAddress: ptr("192.168.1.10"), ExpiresAt: exp},
		{UserID: u.ID, TokenHash: "h-v6", IPAddress: ptr("2001:db8::1"), ExpiresAt: exp},
		{UserID: u.ID, TokenHash: "h-noip", ExpiresAt: exp},
		{UserID: u.ID, TokenHash: "h-expired", ExpiresAt: time.Now().Add(-time.Minute)},
		{UserID: other.ID, TokenHash: "h-other", ExpiresAt: exp},
	}
	for _, s := range sessions {
		if err := repo.Save(ctx, s); err != nil {
			t.Fatalf("save %s: %v", s.TokenHash, err)
		}
	}

	got, err := repo.FindByTokenHash(ctx, "h-v4")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != sessions[0].ID || got.UserID != u.ID || *got.IPAddress != "192.168.1.10" ||
		*got.DeviceName != "Chrome on Linux" || !got.ExpiresAt.Equal(exp) || got.RevokedAt != nil {
		t.Fatalf("find by hash: %+v", got)
	}
	if got, err := repo.FindByID(ctx, sessions[1].ID); err != nil || *got.IPAddress != "2001:db8::1" {
		t.Fatalf("ipv6: %+v %v", got, err)
	}
	if got, err := repo.FindByID(ctx, sessions[2].ID); err != nil || got.IPAddress != nil {
		t.Fatalf("null ip: %+v %v", got, err)
	}
	if _, err := repo.FindByTokenHash(ctx, "missing"); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("missing hash: %v", err)
	}

	// Rotate меняет хэш той же сессии (sid стабилен); устаревший хэш не подходит.
	ok, err := repo.Rotate(ctx, sessions[0].ID, "h-v4", "h-v4-2", exp.Add(time.Hour), ptr("10.0.0.7"), nil, nil)
	if err != nil || !ok {
		t.Fatalf("rotate: %v %v", ok, err)
	}
	if ok, err := repo.Rotate(ctx, sessions[0].ID, "h-v4", "h-v4-3", exp, nil, nil, nil); err != nil || ok {
		t.Fatalf("rotate with stale hash: %v %v", ok, err)
	}
	got, err = repo.FindByTokenHash(ctx, "h-v4-2")
	if err != nil || got.ID != sessions[0].ID || *got.IPAddress != "10.0.0.7" || *got.UserAgent != "Mozilla/5.0" {
		t.Fatalf("after rotate: %+v %v", got, err)
	}
	// nil-поля при ротации не стирают сохранённые значения.
	if ok, _ := repo.Rotate(ctx, sessions[0].ID, "h-v4-2", "h-v4-4", exp, nil, nil, nil); !ok {
		t.Fatal("rotate without client info")
	}
	if got, _ := repo.FindByID(ctx, sessions[0].ID); *got.IPAddress != "10.0.0.7" || *got.DeviceName != "Chrome on Linux" {
		t.Fatalf("client info lost: %+v", got)
	}
	if ok, err := repo.Rotate(ctx, sessions[0].ID, "h-v4-4", "bad", exp, ptr("not-an-ip"), nil, nil); err == nil || ok {
		t.Fatalf("invalid ip accepted: %v %v", ok, err)
	}

	// Активные — только неотозванные и неистёкшие, свои.
	active, err := repo.ListActiveByUser(ctx, u.ID)
	if err != nil || len(active) != 3 {
		t.Fatalf("active: %d %v", len(active), err)
	}

	if err := repo.RevokeByID(ctx, sessions[2].ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.RevokeByID(ctx, sessions[2].ID); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("revoke twice: %v", err)
	}
	if ok, _ := repo.Rotate(ctx, sessions[2].ID, "h-noip", "x", exp, nil, nil, nil); ok {
		t.Fatal("revoked session rotated")
	}
	if got, _ := repo.FindByID(ctx, sessions[2].ID); got.RevokedAt == nil {
		t.Fatal("revoked_at not set")
	}

	// Смена пароля: отзываются все сессии, кроме текущей; чужие не трогаются.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	keep := sessions[0].ID
	revoked, err := repo.RevokeAllByUserTx(ctx, tx, u.ID, &keep)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	// Активна была ещё v6 и истёкшая (отзыв не смотрит на срок).
	if len(revoked) != 2 {
		t.Fatalf("revoked %v", revoked)
	}
	if active, _ := repo.ListActiveByUser(ctx, u.ID); len(active) != 1 || active[0].ID != keep {
		t.Fatalf("after revoke all: %+v", active)
	}
	if active, _ := repo.ListActiveByUser(ctx, other.ID); len(active) != 1 {
		t.Fatal("other user's session revoked")
	}

	// Удаление пользователя удаляет его сессии.
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, other.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindByID(ctx, sessions[4].ID); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("cascade: %v", err)
	}
}

func TestVerificationAndResetTokens(t *testing.T) {
	pool := testutil.Pool(t, "auth_repository_test")
	ver := repository.NewVerificationRepository(pool)
	reset := repository.NewPasswordResetRepository(pool)
	ctx := context.Background()
	u := newUser(t, pool, "dave@example.com")
	exp := time.Now().Add(15 * time.Minute)

	// Повторная отправка письма заменяет токен.
	if err := ver.Create(ctx, u.ID, "v-1", exp); err != nil {
		t.Fatal(err)
	}
	if err := ver.Create(ctx, u.ID, "v-2", exp); err != nil {
		t.Fatal(err)
	}
	if _, err := ver.FindByTokenHash(ctx, "v-1"); !errors.Is(err, apperror.ErrInvalidToken) {
		t.Fatalf("old verification token: %v", err)
	}
	if ev, err := ver.FindByTokenHash(ctx, "v-2"); err != nil || ev.UserID != u.ID {
		t.Fatalf("verification: %+v %v", ev, err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := ver.DeleteByUserIDTx(ctx, tx, u.ID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := ver.FindByTokenHash(ctx, "v-2"); !errors.Is(err, apperror.ErrInvalidToken) {
		t.Fatalf("deleted verification token: %v", err)
	}
	if err := ver.DeleteByUserID(ctx, u.ID); err != nil {
		t.Fatal(err)
	}

	if err := reset.Create(ctx, u.ID, "r-1", exp); err != nil {
		t.Fatal(err)
	}
	if prt, err := reset.FindByTokenHash(ctx, "r-1"); err != nil || prt.UserID != u.ID {
		t.Fatalf("reset: %+v %v", prt, err)
	}
	if _, err := reset.FindByTokenHash(ctx, "r-x"); !errors.Is(err, apperror.ErrInvalidToken) {
		t.Fatalf("unknown reset token: %v", err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := reset.DeleteByUserIDTx(ctx, tx, u.ID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := reset.FindByTokenHash(ctx, "r-1"); !errors.Is(err, apperror.ErrInvalidToken) {
		t.Fatalf("deleted reset token: %v", err)
	}
}
