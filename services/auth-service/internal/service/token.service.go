package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/auth-service/config"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/events"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/utils"
	"github.com/maltira/chavo-project-backend/services/auth-service/pkg/kafka"
)

const (
	SessionKeyPrefix = "ws:session:"
)

type TokenService interface {
	GenerateTokens(ctx context.Context, userID uuid.UUID, ip, userAgent, device string) (accessToken, refreshToken string, err error)
	Refresh(ctx context.Context, refreshToken string, ip, userAgent, device string) (accessToken, newRefreshToken string, err error)
	RevokeCurrent(ctx context.Context, userID uuid.UUID, refreshToken string) error
	RevokeByID(ctx context.Context, userID, sessionID uuid.UUID) error
	ListActiveSessions(ctx context.Context, userID uuid.UUID) ([]models.RefreshToken, error)
}

type tokenService struct {
	repo     repository.TokenRepository
	rdb      *redis.Client
	producer *kafka.Producer
	cfg      *config.Config
	log      *zap.Logger
}

func NewTokenService(
	repo repository.TokenRepository,
	rdb *redis.Client,
	producer *kafka.Producer,
	cfg *config.Config,
	log *zap.Logger,
) TokenService {
	return &tokenService{
		repo:     repo,
		rdb:      rdb,
		producer: producer,
		cfg:      cfg,
		log:      log,
	}
}

func (s *tokenService) GenerateTokens(ctx context.Context, userID uuid.UUID, ip, userAgent, device string) (string, string, error) {
	plainRefreshToken, expiresAt, err := utils.GenerateRefreshToken(s.cfg.RefreshTokenDuration)
	if err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	sessionID := uuid.New()
	tokenHash := utils.HashSHA256(plainRefreshToken)

	var ipPtr, uaPtr, devPtr *string
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

	rt := &models.RefreshToken{
		ID:         sessionID,
		UserID:     userID,
		TokenHash:  tokenHash,
		DeviceName: devPtr,
		UserAgent:  uaPtr,
		IPAddress:  ipPtr,
		ExpiresAt:  expiresAt,
	}

	if err = s.repo.Save(ctx, rt); err != nil {
		return "", "", fmt.Errorf("save refresh token to db: %w", err)
	}

	accessToken, err := utils.GenerateAccessToken(userID, sessionID, s.cfg.JWTSecret, s.cfg.AccessTokenDuration)
	if err != nil {
		return "", "", fmt.Errorf("generate access token: %w", err)
	}

	// Сохраняем активную сессию в Redis (whitelist)
	sessionKey := SessionKeyPrefix + sessionID.String()
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		ttl = s.cfg.RefreshTokenDuration
	}

	if err = s.rdb.Set(ctx, sessionKey, userID.String(), ttl).Err(); err != nil {
		s.log.Error("Failed to cache session in Redis", zap.String("session_id", sessionID.String()), zap.Error(err))
	}

	return accessToken, plainRefreshToken, nil
}

func (s *tokenService) Refresh(ctx context.Context, refreshToken string, ip, userAgent, device string) (string, string, error) {
	tokenHash := utils.HashSHA256(refreshToken)
	rt, err := s.repo.FindByTokenHash(ctx, tokenHash)
	if err != nil {
		return "", "", apperror.ErrInvalidToken
	}

	if rt.RevokedAt != nil || time.Now().After(rt.ExpiresAt) {
		return "", "", apperror.ErrInvalidToken
	}

	// Инвалидируем старую сессию
	_ = s.repo.RevokeByID(ctx, rt.ID)
	_ = s.rdb.Del(ctx, SessionKeyPrefix+rt.ID.String()).Err()

	// Выпускаем новую пару токенов
	return s.GenerateTokens(ctx, rt.UserID, ip, userAgent, device)
}

func (s *tokenService) RevokeCurrent(ctx context.Context, userID uuid.UUID, refreshToken string) error {
	tokenHash := utils.HashSHA256(refreshToken)
	rt, err := s.repo.FindByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			return nil
		}
		return err
	}

	if rt.UserID != userID {
		return apperror.ErrForbidden
	}

	return s.RevokeByID(ctx, userID, rt.ID)
}

func (s *tokenService) RevokeByID(ctx context.Context, userID, sessionID uuid.UUID) error {
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

	if err = s.repo.RevokeByID(ctx, sessionID); err != nil {
		return err
	}

	// Удаляем из Redis
	_ = s.rdb.Del(ctx, SessionKeyPrefix+sessionID.String()).Err()

	// Публикуем событие в Kafka
	s.publishRevokedEvent(ctx, sessionID, userID, "remote_logout")

	return nil
}

func (s *tokenService) ListActiveSessions(ctx context.Context, userID uuid.UUID) ([]models.RefreshToken, error) {
	return s.repo.ListActiveByUser(ctx, userID)
}

func (s *tokenService) publishRevokedEvent(ctx context.Context, sessionID, userID uuid.UUID, reason string) {
	if s.producer == nil {
		return
	}

	ev := events.NewEvent(events.TypeSessionRevoked, events.SessionRevokedPayload{
		SessionID: sessionID,
		UserID:    userID,
		Reason:    reason,
	})

	if err := s.producer.Publish(ctx, events.TopicAuthEvents, userID.String(), ev); err != nil {
		s.log.Error("Failed to publish session.revoked event",
			zap.String("session_id", sessionID.String()),
			zap.Error(err),
		)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
