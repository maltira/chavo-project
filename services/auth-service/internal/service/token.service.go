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
	if err = s.rdb.Set(ctx, SessionKeyPrefix+sessionID.String(), userID.String(), ttl).Err(); err != nil {
		s.log.Error("Failed to cache session in Redis", zap.String("session_id", sessionID.String()), zap.Error(err))
	}

	// страховка от гонки с удалением аккаунта между Save/Rotate и записью ключа в Redis
	if current, err := s.repo.FindByID(ctx, sessionID); err == nil && current.RevokedAt != nil {
		_ = s.rdb.Del(ctx, SessionKeyPrefix+sessionID.String()).Err()
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
	if s.rdb != nil {
		_ = s.rdb.Del(ctx, SessionKeyPrefix+sessionID.String()).Err()
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
