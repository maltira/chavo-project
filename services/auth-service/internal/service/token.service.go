package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/auth-service/config"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/events"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/utils"
)

const (
	SessionKeyPrefix = "ws:session:"
)

var redisDeleteBackoff = []time.Duration{0, 25 * time.Millisecond, 75 * time.Millisecond}

// sessionRedisKeys строит список Redis-ключей для набора идентификаторов сессий.
func sessionRedisKeys(ids ...uuid.UUID) []string {
	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, SessionKeyPrefix+id.String())
	}
	return keys
}

// deleteRedisKeys удаляет ключи из Redis с короткими повторами при временном сбое.
func deleteRedisKeys(ctx context.Context, rdb *redis.Client, log *zap.Logger, keys ...string) error {
	if rdb == nil || len(keys) == 0 {
		return nil
	}
	var err error
	for i, wait := range redisDeleteBackoff {
		if wait > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}
		if err = rdb.Del(ctx, keys...).Err(); err == nil {
			return nil
		}
		if log != nil {
			log.Warn("Failed to delete Redis keys",
				zap.Int("attempt", i+1),
				zap.Strings("keys", keys),
				zap.Error(err),
			)
		}
	}
	if log != nil {
		log.Error("Exhausted retries deleting Redis keys", zap.Strings("keys", keys), zap.Error(err))
	}
	return err
}

// Tokens — результат входа или обновления; SessionID (sid) стабилен на всё время сессии устройства.
type Tokens struct {
	AccessToken  string
	RefreshToken string
	UserID       uuid.UUID
	SessionID    uuid.UUID
	ExpiresAt    time.Time
}

type TokenService interface {
	GenerateTokens(ctx context.Context, userID uuid.UUID, ip, userAgent, device string) (*Tokens, error)
	// Refresh ротирует refresh-токен в той же сессии: sid не меняется.
	Refresh(ctx context.Context, refreshToken string, ip, userAgent, device string) (*Tokens, error)
	// Resolve проверяет refresh-токен без ротации и возвращает владельца и sid.
	Resolve(ctx context.Context, refreshToken string) (userID, sessionID uuid.UUID, err error)
	// RevokeByToken завершает сессию по refresh-токену; неизвестный или уже отозванный токен — не ошибка.
	RevokeByToken(ctx context.Context, refreshToken string) error
	RevokeByID(ctx context.Context, userID, sessionID uuid.UUID) error
	ListActiveSessions(ctx context.Context, userID uuid.UUID) ([]models.RefreshToken, error)
}

type tokenService struct {
	repo         repository.TokenRepository
	outboxRepo   repository.OutboxRepository
	pool         *pgxpool.Pool
	rdb          *redis.Client
	notifyOutbox func()
	cfg          *config.Config
	log          *zap.Logger
}

func NewTokenService(
	repo repository.TokenRepository,
	outboxRepo repository.OutboxRepository,
	pool *pgxpool.Pool,
	rdb *redis.Client,
	notifyOutbox func(),
	cfg *config.Config,
	log *zap.Logger,
) TokenService {
	return &tokenService{
		repo:         repo,
		outboxRepo:   outboxRepo,
		pool:         pool,
		rdb:          rdb,
		notifyOutbox: notifyOutbox,
		cfg:          cfg,
		log:          log,
	}
}

func clientInfo(ip, userAgent, device string) (ipPtr, uaPtr, devPtr *string) {
	if ip != "" {
		ipPtr = &ip
	}
	if userAgent != "" {
		truncatedUA := truncate(userAgent, 254)
		uaPtr = &truncatedUA
	}
	if device != "" {
		devPtr = &device
	}
	return ipPtr, uaPtr, devPtr
}

func (s *tokenService) GenerateTokens(ctx context.Context, userID uuid.UUID, ip, userAgent, device string) (*Tokens, error) {
	plainRefreshToken, expiresAt, err := utils.GenerateRefreshToken(s.cfg.RefreshTokenDuration)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	ipPtr, uaPtr, devPtr := clientInfo(ip, userAgent, device)
	rt := &models.RefreshToken{
		ID:         uuid.New(),
		UserID:     userID,
		TokenHash:  utils.HashSHA256(plainRefreshToken),
		DeviceName: devPtr,
		UserAgent:  uaPtr,
		IPAddress:  ipPtr,
		ExpiresAt:  expiresAt,
	}
	if err = s.repo.Save(ctx, rt); err != nil {
		if errors.Is(err, apperror.ErrUnauthorized) {
			return nil, err
		}
		return nil, fmt.Errorf("save refresh token to db: %w", err)
	}
	return s.issue(ctx, rt.UserID, rt.ID, plainRefreshToken, expiresAt)
}

// issue выпускает access token для сессии и продлевает её запись в Redis (whitelist для Gateway).
func (s *tokenService) issue(ctx context.Context, userID, sessionID uuid.UUID, refreshToken string, expiresAt time.Time) (*Tokens, error) {
	accessToken, err := utils.GenerateAccessToken(userID, sessionID, s.cfg.JWTSecret, s.cfg.AccessTokenDuration)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		ttl = s.cfg.RefreshTokenDuration
	}
	cached := false
	if s.rdb != nil {
		if err = s.rdb.Set(ctx, SessionKeyPrefix+sessionID.String(), userID.String(), ttl).Err(); err != nil {
			s.log.Error("Failed to cache session in Redis", zap.String("session_id", sessionID.String()), zap.Error(err))
		} else {
			cached = true
		}
	}

	// страховка от гонки с удалением аккаунта или отзывом между Save/Rotate и записью ключа в Redis
	current, err := s.repo.FindByID(ctx, sessionID)
	if err != nil {
		if cached {
			_ = deleteRedisKeys(ctx, s.rdb, s.log, SessionKeyPrefix+sessionID.String())
		}
		return nil, fmt.Errorf("verify session after cache: %w", err)
	}
	if current.RevokedAt != nil {
		if cached {
			if delErr := deleteRedisKeys(ctx, s.rdb, s.log, SessionKeyPrefix+sessionID.String()); delErr != nil {
				return nil, fmt.Errorf("delete revoked session from redis: %w", delErr)
			}
		}
		return nil, apperror.ErrUnauthorized
	}

	return &Tokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       userID,
		SessionID:    sessionID,
		ExpiresAt:    expiresAt,
	}, nil
}

// activeByToken — действующая (не отозванная и не истёкшая) сессия по refresh-токену.
func (s *tokenService) activeByToken(ctx context.Context, refreshToken string) (*models.RefreshToken, error) {
	if refreshToken == "" {
		return nil, apperror.ErrInvalidToken
	}
	rt, err := s.repo.FindByTokenHash(ctx, utils.HashSHA256(refreshToken))
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			return nil, apperror.ErrInvalidToken
		}
		return nil, err
	}
	if rt.RevokedAt != nil || time.Now().After(rt.ExpiresAt) {
		return nil, apperror.ErrInvalidToken
	}
	return rt, nil
}

func (s *tokenService) Refresh(ctx context.Context, refreshToken string, ip, userAgent, device string) (*Tokens, error) {
	rt, err := s.activeByToken(ctx, refreshToken)
	if err != nil {
		return nil, err
	}

	plain, expiresAt, err := utils.GenerateRefreshToken(s.cfg.RefreshTokenDuration)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}
	ipPtr, uaPtr, devPtr := clientInfo(ip, userAgent, device)

	// Условие по старому хэшу: из двух одновременных refresh одним токеном пройдёт только один.
	rotated, err := s.repo.Rotate(ctx, rt.ID, rt.TokenHash, utils.HashSHA256(plain), expiresAt, ipPtr, uaPtr, devPtr)
	if err != nil {
		return nil, fmt.Errorf("rotate refresh token: %w", err)
	}
	if !rotated {
		return nil, apperror.ErrInvalidToken
	}
	return s.issue(ctx, rt.UserID, rt.ID, plain, expiresAt)
}

func (s *tokenService) Resolve(ctx context.Context, refreshToken string) (uuid.UUID, uuid.UUID, error) {
	rt, err := s.activeByToken(ctx, refreshToken)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return rt.UserID, rt.ID, nil
}

func (s *tokenService) RevokeByToken(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	rt, err := s.repo.FindByTokenHash(ctx, utils.HashSHA256(refreshToken))
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			return nil
		}
		return err
	}
	if rt.RevokedAt != nil {
		// сессия уже отозвана в БД - дочищаем ключ в Redis (например, при повторе после сбоя)
		if err = deleteRedisKeys(ctx, s.rdb, s.log, SessionKeyPrefix+rt.ID.String()); err != nil {
			return fmt.Errorf("delete revoked session from redis: %w", err)
		}
		return nil
	}
	return s.revoke(ctx, rt.UserID, rt.ID, "logout")
}

// RevokeByID завершает сессию из списка устройств (с другого устройства или вкладки).
func (s *tokenService) RevokeByID(ctx context.Context, userID, sessionID uuid.UUID) error {
	return s.revoke(ctx, userID, sessionID, "remote_logout")
}

// reason уходит в session.revoked: клиент отличает собственный выход от завершения сессии с другого устройства.
func (s *tokenService) revoke(ctx context.Context, userID, sessionID uuid.UUID, reason string) error {
	rt, err := s.repo.FindByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			return nil
		}
		return err
	}

	if rt.UserID != userID {
		return apperror.ErrForbidden
	}

	if rt.RevokedAt != nil {
		if err = deleteRedisKeys(ctx, s.rdb, s.log, SessionKeyPrefix+sessionID.String()); err != nil {
			return fmt.Errorf("delete revoked session from redis: %w", err)
		}
		return nil
	}

	if s.pool != nil && s.outboxRepo != nil {
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin revoke tx: %w", err)
		}
		defer tx.Rollback(ctx)

		if err = s.repo.RevokeByIDTx(ctx, tx, sessionID); err != nil {
			return err
		}
		ev := events.NewEvent(events.TypeSessionRevoked, events.SessionRevokedPayload{
			SessionID: sessionID,
			UserID:    userID,
			Reason:    reason,
		})
		if err = s.outboxRepo.InsertTx(ctx, tx, events.TopicAuthEvents, userID.String(), ev); err != nil {
			return err
		}
		if err = tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit revoke tx: %w", err)
		}
		if s.notifyOutbox != nil {
			s.notifyOutbox()
		}
	} else {
		if err = s.repo.RevokeByID(ctx, sessionID); err != nil {
			return err
		}
	}

	// удаляем из Redis
	if err = deleteRedisKeys(ctx, s.rdb, s.log, SessionKeyPrefix+sessionID.String()); err != nil {
		return fmt.Errorf("delete session from redis: %w", err)
	}

	return nil
}

func (s *tokenService) ListActiveSessions(ctx context.Context, userID uuid.UUID) ([]models.RefreshToken, error) {
	return s.repo.ListActiveByUser(ctx, userID)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
