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
	mu           sync.Mutex
	rows         map[uuid.UUID]*models.RefreshToken
	findByIDErr  error
	revokeOnRead bool
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
	if m.findByIDErr != nil {
		return nil, m.findByIDErr
	}
	r, ok := m.rows[id]
	if !ok {
		return nil, apperror.ErrNotFound
	}
	if m.revokeOnRead {
		now := time.Now()
		r.RevokedAt = &now
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

func (m *memTokens) RevokeByIDTx(ctx context.Context, _ pgx.Tx, id uuid.UUID) error {
	return m.RevokeByID(ctx, id)
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
	rdb, _ := newFakeRedis()
	cfg := &config.Config{JWTSecret: "test", AccessTokenDuration: time.Minute, RefreshTokenDuration: time.Hour}
	return NewTokenService(repo, nil, nil, rdb, nil, cfg, zap.NewNop())
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

type fakeRedisHook struct {
	mu       sync.Mutex
	store    map[string]string
	delErrs  []error
	delCalls int
}

func newFakeRedis() (*redis.Client, *fakeRedisHook) {
	hook := &fakeRedisHook{store: map[string]string{}}
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1})
	rdb.AddHook(hook)
	return rdb, hook
}

func (h *fakeRedisHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h *fakeRedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func (h *fakeRedisHook) ProcessHook(redis.ProcessHook) redis.ProcessHook {
	return func(_ context.Context, cmd redis.Cmder) error {
		h.mu.Lock()
		defer h.mu.Unlock()
		switch cmd.Name() {
		case "set":
			args := cmd.Args()
			key := args[1].(string)
			val := args[2].(string)
			if bc, ok := cmd.(*redis.BoolCmd); ok {
				_, exists := h.store[key]
				if !exists {
					h.store[key] = val
				}
				bc.SetVal(!exists)
				return nil
			}
			h.store[key] = val
			return nil
		case "setnx":
			args := cmd.Args()
			key := args[1].(string)
			_, exists := h.store[key]
			if !exists {
				h.store[key] = args[2].(string)
			}
			if bc, ok := cmd.(*redis.BoolCmd); ok {
				bc.SetVal(!exists)
			}
			return nil
		case "del":
			h.delCalls++
			if len(h.delErrs) > 0 {
				err := h.delErrs[0]
				h.delErrs = h.delErrs[1:]
				if err != nil {
					cmd.SetErr(err)
					return err
				}
			}
			for _, arg := range cmd.Args()[1:] {
				delete(h.store, arg.(string))
			}
			return nil
		default:
			return nil
		}
	}
}

func TestIssueFailsClosedOnFindByIDError(t *testing.T) {
	repo := &memTokens{rows: map[uuid.UUID]*models.RefreshToken{}, findByIDErr: errors.New("db down")}
	rdb, hook := newFakeRedis()
	cfg := &config.Config{JWTSecret: "test", AccessTokenDuration: time.Minute, RefreshTokenDuration: time.Hour}
	svc := NewTokenService(repo, nil, nil, rdb, nil, cfg, zap.NewNop())

	if _, err := svc.GenerateTokens(context.Background(), uuid.New(), "", "", ""); err == nil {
		t.Fatal("GenerateTokens must fail closed when final FindByID check errors")
	}
	if len(hook.store) != 0 {
		t.Fatalf("session key must be removed from Redis on FindByID failure, left: %v", hook.store)
	}
}

func TestIssueRetriesAndErrorsWhenRedisDeleteFailsForRevokedSession(t *testing.T) {
	t.Run("transient delete error succeeds on retry", func(t *testing.T) {
		repo := &memTokens{rows: map[uuid.UUID]*models.RefreshToken{}, revokeOnRead: true}
		rdb, hook := newFakeRedis()
		hook.delErrs = []error{errors.New("redis blip"), nil}
		cfg := &config.Config{JWTSecret: "test", AccessTokenDuration: time.Minute, RefreshTokenDuration: time.Hour}
		svc := NewTokenService(repo, nil, nil, rdb, nil, cfg, zap.NewNop())

		_, err := svc.GenerateTokens(context.Background(), uuid.New(), "", "", "")
		if !errors.Is(err, apperror.ErrUnauthorized) {
			t.Fatalf("err = %v, want ErrUnauthorized", err)
		}
		if len(hook.store) != 0 {
			t.Fatalf("revoked session key must be deleted after retry, left: %v", hook.store)
		}
	})

	t.Run("persistent delete error returns error", func(t *testing.T) {
		repo := &memTokens{rows: map[uuid.UUID]*models.RefreshToken{}, revokeOnRead: true}
		rdb, hook := newFakeRedis()
		hook.delErrs = []error{errors.New("redis down"), errors.New("redis down"), errors.New("redis down")}
		cfg := &config.Config{JWTSecret: "test", AccessTokenDuration: time.Minute, RefreshTokenDuration: time.Hour}
		svc := NewTokenService(repo, nil, nil, rdb, nil, cfg, zap.NewNop())

		_, err := svc.GenerateTokens(context.Background(), uuid.New(), "", "", "")
		if err == nil || errors.Is(err, apperror.ErrUnauthorized) {
			t.Fatalf("persistent redis delete failure must surface non-unauthorized error, got: %v", err)
		}
	})
}

func TestRevokeReturnsErrorOnRedisFailureAndCleansOnRetry(t *testing.T) {
	repo := &memTokens{rows: map[uuid.UUID]*models.RefreshToken{}}
	rdb, hook := newFakeRedis()
	cfg := &config.Config{JWTSecret: "test", AccessTokenDuration: time.Minute, RefreshTokenDuration: time.Hour}
	svc := NewTokenService(repo, nil, nil, rdb, nil, cfg, zap.NewNop())
	ctx := context.Background()

	tokens, err := svc.GenerateTokens(ctx, uuid.New(), "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(hook.store) != 1 {
		t.Fatalf("expected session in redis, got %v", hook.store)
	}

	// все попытки DEL падают - отзыв должен вернуть ошибку, а не промолчать
	hook.delErrs = []error{errors.New("redis down"), errors.New("redis down"), errors.New("redis down")}
	if err = svc.RevokeByToken(ctx, tokens.RefreshToken); err == nil {
		t.Fatal("RevokeByToken must return error when Redis delete fails")
	}

	// повтор вызова после восстановления Redis должен дочистить ключ, хотя в БД сессия уже revoked
	if err = svc.RevokeByToken(ctx, tokens.RefreshToken); err != nil {
		t.Fatalf("retry RevokeByToken failed: %v", err)
	}
	if len(hook.store) != 0 {
		t.Fatalf("session key must be cleaned on retry, left: %v", hook.store)
	}
}
