package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/maltira/chavo-project-backend/services/auth-service/config"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/auth-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/utils"
)

type TokenService interface {
	GenerateTokens(ctx context.Context, userID uuid.UUID, ip, userAgent, device string) (accessToken, refreshToken string, err error)
	Refresh(ctx context.Context, refreshToken string) (accessToken, newRefreshToken string, err error)
	RevokeByToken(ctx context.Context, refreshToken string) error
	RevokeByID(ctx context.Context, userID, tokenID uuid.UUID) error
	RevokeAll(ctx context.Context, userID uuid.UUID, excludeToken *string) error
	ListActiveSessions(ctx context.Context, userID uuid.UUID) ([]models.RefreshToken, error)
}

type tokenService struct {
	repo repository.TokenRepository
	rdb  *redis.Client
	cfg  *config.Config
	log  *zap.Logger
}

func NewTokenService(
	repo repository.TokenRepository,
	rdb *redis.Client,
	cfg *config.Config,
	log *zap.Logger,
) TokenService {
	return &tokenService{repo: repo, rdb: rdb, cfg: cfg, log: log}
}

// GenerateTokens creates a new access + refresh token pair and persists the refresh token.
func (s *tokenService) GenerateTokens(ctx context.Context, userID uuid.UUID, ip, userAgent, device string) (string, string, error) {
	accessToken, jti, err := utils.GenerateAccessToken(userID, s.cfg.JWTSecret, s.cfg.AccessTokenDuration)
	if err != nil {
		return "", "", err
	}

	refreshToken, expiresAt, err := utils.GenerateRefreshToken(s.cfg.RefreshTokenDuration)
	if err != nil {
		return "", "", err
	}

	rt := &models.RefreshToken{
		UserID:    userID,
		Token:     refreshToken,
		AccessJTI: jti,
		IP:        ip,
		UserAgent: truncate(userAgent, 254),
		Device:    device,
		ExpiresAt: expiresAt,
	}

	if err = s.repo.Save(ctx, rt); err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// Refresh rotates a refresh token: validates the old one, blacklists its access JTI,
// deletes it, and issues a new pair.
func (s *tokenService) Refresh(ctx context.Context, refreshToken string) (string, string, error) {
	rt, err := s.repo.FindByToken(ctx, refreshToken)
	if err != nil || time.Now().After(rt.ExpiresAt) {
		return "", "", apperror.ErrInvalidToken
	}

	// Blacklist the old access token's JTI.
	s.blacklistJTI(ctx, rt.AccessJTI)

	// Delete the old refresh token.
	_ = s.repo.Delete(ctx, refreshToken)

	// Generate a new pair.
	return s.GenerateTokens(ctx, rt.UserID, rt.IP, rt.UserAgent, rt.Device)
}

// RevokeByToken revokes a single refresh token by its value.
func (s *tokenService) RevokeByToken(ctx context.Context, refreshToken string) error {
	rt, err := s.repo.FindByToken(ctx, refreshToken)
	if err == nil {
		s.blacklistJTI(ctx, rt.AccessJTI)
	}
	return s.repo.Delete(ctx, refreshToken)
}

// RevokeByID revokes a refresh token by its DB ID, ensuring ownership.
func (s *tokenService) RevokeByID(ctx context.Context, userID, tokenID uuid.UUID) error {
	rt, err := s.repo.FindByID(ctx, tokenID)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			return nil // already gone — idempotent
		}
		return err
	}

	if rt.UserID != userID {
		return apperror.ErrForbidden
	}

	s.blacklistJTI(ctx, rt.AccessJTI)
	return s.repo.Delete(ctx, rt.Token)
}

// RevokeAll revokes all refresh tokens for a user, optionally excluding one.
func (s *tokenService) RevokeAll(ctx context.Context, userID uuid.UUID, excludeToken *string) error {
	jtis, err := s.repo.DeleteAllByUser(ctx, userID, excludeToken)
	if err != nil {
		return fmt.Errorf("delete all tokens for user %s: %w", userID, err)
	}
	for _, jti := range jtis {
		s.blacklistJTI(ctx, jti)
	}
	return nil
}

// ListActiveSessions returns active sessions for a user.
func (s *tokenService) ListActiveSessions(ctx context.Context, userID uuid.UUID) ([]models.RefreshToken, error) {
	return s.repo.ListActiveByUser(ctx, userID)
}

// blacklistJTI adds a JTI to the Redis blacklist with TTL = AccessTokenDuration.
func (s *tokenService) blacklistJTI(ctx context.Context, jti string) {
	if jti == "" {
		return
	}
	key := "blacklist:jti:" + jti
	if err := s.rdb.Set(ctx, key, "1", s.cfg.AccessTokenDuration).Err(); err != nil {
		s.log.Warn("Failed to blacklist JTI", zap.String("jti", jti), zap.Error(err))
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
