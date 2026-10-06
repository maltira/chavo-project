package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/auth-service/config"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/models"
)

// memTokens — потокобезопасная in-memory реализация TokenRepository.
type memTokens struct {
	mu   sync.Mutex
	rows map[uuid.UUID]*models.RefreshToken
}

func (m *memTokens) Save(_ context.Context, t *models.RefreshToken) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := *t
	c.CreatedAt = time.Now()
	m.rows[t.ID] = &c
	return nil
}

func (m *memTokens) FindByTokenHash(_ context.Context, hash string) (*models.RefreshToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.rows {
		if r.TokenHash == hash {
			c := *r
			return &c, nil
		}
	}
	return nil, apperror.ErrNotFound
}

func (m *memTokens) FindByID(_ context.Context, id uuid.UUID) (*models.RefreshToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[id]
	if !ok {
		return nil, apperror.ErrNotFound
	}
	c := *r
	return &c, nil
}

func (m *memTokens) RevokeByID(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	m.rows[id].RevokedAt = &now
	return nil
}

func (m *memTokens) Rotate(_ context.Context, id uuid.UUID, oldHash, newHash string, exp time.Time, _, _, _ *string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[id]
	if !ok || r.TokenHash != oldHash || r.RevokedAt != nil {
		return false, nil
	}
	r.TokenHash, r.ExpiresAt = newHash, exp
	return true, nil
}

func (m *memTokens) RevokeAllByUserTx(context.Context, pgx.Tx, uuid.UUID, *uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}

func (m *memTokens) ListActiveByUser(context.Context, uuid.UUID) ([]models.RefreshToken, error) {
	return nil, nil
}

func newTestTokenService(repo *memTokens) TokenService {
	// Redis недоступен: запись whitelist только логируется, логика сессий от неё не зависит.
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: 50 * time.Millisecond, MaxRetries: -1})
	cfg := &config.Config{JWTSecret: "test", AccessTokenDuration: time.Minute, RefreshTokenDuration: time.Hour}
	return NewTokenService(repo, rdb, nil, cfg, zap.NewNop())
}

func TestRefreshKeepsSessionID(t *testing.T) {
	repo := &memTokens{rows: map[uuid.UUID]*models.RefreshToken{}}
	svc := newTestTokenService(repo)
	ctx := context.Background()
	user := uuid.New()

	first, err := svc.GenerateTokens(ctx, user, "1.2.3.4", "ua", "dev")
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Refresh(ctx, first.RefreshToken, "1.2.3.4", "ua", "dev")
	if err != nil {
		t.Fatal(err)
	}
	third, err := svc.Refresh(ctx, second.RefreshToken, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if second.SessionID != first.SessionID || third.SessionID != first.SessionID || len(repo.rows) != 1 {
		t.Fatalf("sid must survive refresh: %s %s %s (rows=%d)", first.SessionID, second.SessionID, third.SessionID, len(repo.rows))
	}
	if first.RefreshToken == second.RefreshToken || second.RefreshToken == third.RefreshToken {
		t.Fatal("refresh token must rotate")
	}
	if _, err = svc.Refresh(ctx, first.RefreshToken, "", "", ""); !errors.Is(err, apperror.ErrInvalidToken) {
		t.Fatalf("rotated-out token must be rejected: %v", err)
	}

	u, sid, err := svc.Resolve(ctx, third.RefreshToken)
	if err != nil || u != user || sid != first.SessionID {
		t.Fatalf("resolve: %s %s %v", u, sid, err)
	}
	if _, _, err = svc.Resolve(ctx, second.RefreshToken); !errors.Is(err, apperror.ErrInvalidToken) {
		t.Fatalf("resolve old token: %v", err)
	}
}

func TestConcurrentRefreshWithSameTokenSucceedsOnce(t *testing.T) {
	repo := &memTokens{rows: map[uuid.UUID]*models.RefreshToken{}}
	svc := newTestTokenService(repo)
	ctx := context.Background()
	start, _ := svc.GenerateTokens(ctx, uuid.New(), "", "", "")

	var (
		wg sync.WaitGroup
		mu sync.Mutex
		ok int
	)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.Refresh(ctx, start.RefreshToken, "", "", ""); err == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if ok != 1 {
		t.Fatalf("successful refreshes = %d, want 1", ok)
	}
}

func TestRevokeByToken(t *testing.T) {
	repo := &memTokens{rows: map[uuid.UUID]*models.RefreshToken{}}
	svc := newTestTokenService(repo)
	ctx := context.Background()
	tokens, _ := svc.GenerateTokens(ctx, uuid.New(), "", "", "")

	if err := svc.RevokeByToken(ctx, tokens.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Resolve(ctx, tokens.RefreshToken); !errors.Is(err, apperror.ErrInvalidToken) {
		t.Fatalf("revoked session must not resolve: %v", err)
	}
	if _, err := svc.Refresh(ctx, tokens.RefreshToken, "", "", ""); !errors.Is(err, apperror.ErrInvalidToken) {
		t.Fatalf("revoked session must not refresh: %v", err)
	}
	for _, tok := range []string{tokens.RefreshToken, "unknown", ""} {
		if err := svc.RevokeByToken(ctx, tok); err != nil {
			t.Fatalf("repeat/unknown logout must be a no-op: %v", err)
		}
	}
}
