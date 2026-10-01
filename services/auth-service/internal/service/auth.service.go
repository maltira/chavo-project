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
	"golang.org/x/crypto/bcrypt"

	"github.com/maltira/chavo-project-backend/services/auth-service/config"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/email"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/events"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/utils"
	"github.com/maltira/chavo-project-backend/services/auth-service/pkg/kafka"
)

type AuthService interface {
	Register(ctx context.Context, email, password string) error
	VerifyRegister(ctx context.Context, token string) error
	Login(ctx context.Context, email, password string) (challengeID string, err error)
	RequestPasswordReset(ctx context.Context, email string) error
	ConfirmPasswordReset(ctx context.Context, token, newPassword string) error
	ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error
	FindByID(ctx context.Context, id uuid.UUID) (*models.User, error)
}

type authService struct {
	userRepo  repository.UserRepository
	verRepo   repository.VerificationRepository
	resetRepo repository.PasswordResetRepository
	tokenRepo repository.TokenRepository
	otpSvc    OtpService
	pool      *pgxpool.Pool
	rdb       *redis.Client
	producer  *kafka.Producer
	mail      email.EmailSender
	cfg       *config.Config
	log       *zap.Logger
}

func NewAuthService(
	userRepo repository.UserRepository,
	verRepo repository.VerificationRepository,
	resetRepo repository.PasswordResetRepository,
	tokenRepo repository.TokenRepository,
	otpSvc OtpService,
	pool *pgxpool.Pool,
	rdb *redis.Client,
	producer *kafka.Producer,
	mail email.EmailSender,
	cfg *config.Config,
	log *zap.Logger,
) AuthService {
	return &authService{
		userRepo:  userRepo,
		verRepo:   verRepo,
		resetRepo: resetRepo,
		tokenRepo: tokenRepo,
		otpSvc:    otpSvc,
		pool:      pool,
		rdb:       rdb,
		producer:  producer,
		mail:      mail,
		cfg:       cfg,
		log:       log,
	}
}

func (s *authService) Register(ctx context.Context, email, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	user, err := s.userRepo.CreateOrUpdateUnverified(ctx, email, string(hash))
	if err != nil {
		return err
	}

	plainToken, err := utils.GenerateSecureToken(32)
	if err != nil {
		return fmt.Errorf("generate verification token: %w", err)
	}

	tokenHash := utils.HashSHA256(plainToken)
	expiresAt := time.Now().Add(s.cfg.VerificationTTL)

	if err = s.verRepo.Create(ctx, user.ID, tokenHash, expiresAt); err != nil {
		return fmt.Errorf("save verification token: %w", err)
	}

	// отправляем ссылку-верификацию на фоне
	verifyURL := fmt.Sprintf("%s/auth/register/verify?token=%s", s.cfg.FrontendURL, plainToken)
	utils.SafeGo(s.log, "send verification email", func() {
		if err := s.mail.SendVerification(email, verifyURL); err != nil {
			s.log.Error("Failed to send verification email",
				zap.String("email", email), zap.Error(err))
		}
	})

	return nil
}

func (s *authService) VerifyRegister(ctx context.Context, token string) error {
	tokenHash := utils.HashSHA256(token)
	ev, err := s.verRepo.FindByTokenHash(ctx, tokenHash)
	if err != nil {
		return apperror.ErrInvalidToken
	}

	if time.Now().After(ev.ExpiresAt) {
		_ = s.verRepo.DeleteByUserID(ctx, ev.UserID)
		return apperror.ErrInvalidToken
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err = s.userRepo.SetVerifiedTx(ctx, tx, ev.UserID); err != nil {
		return fmt.Errorf("set verified: %w", err)
	}

	if err = s.verRepo.DeleteByUserIDTx(ctx, tx, ev.UserID); err != nil {
		return fmt.Errorf("delete verification token: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (s *authService) Login(ctx context.Context, email, password string) (string, error) {
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			return "", apperror.ErrInvalidCredentials
		}
		return "", err
	}

	if !user.EmailVerified {
		return "", apperror.ErrAccountNotVerified
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return "", apperror.ErrInvalidCredentials
	}

	// Пароль верный -> создаем OTP challenge
	challengeID, err := s.otpSvc.CreateChallenge(ctx, user.ID, user.Email)
	if err != nil {
		return "", fmt.Errorf("create otp challenge: %w", err)
	}

	return challengeID, nil
}

func (s *authService) RequestPasswordReset(ctx context.Context, email string) error {
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			return nil
		}
		return err
	}

	if !user.EmailVerified {
		return nil
	}

	plainToken, err := utils.GenerateSecureToken(32)
	if err != nil {
		return fmt.Errorf("generate reset token: %w", err)
	}

	tokenHash := utils.HashSHA256(plainToken)
	expiresAt := time.Now().Add(s.cfg.VerificationTTL)

	if err = s.resetRepo.Create(ctx, user.ID, tokenHash, expiresAt); err != nil {
		return fmt.Errorf("save reset token: %w", err)
	}

	resetURL := fmt.Sprintf("%s/auth/reset/confirm?token=%s", s.cfg.FrontendURL, plainToken)
	utils.SafeGo(s.log, "send reset password email", func() {
		if err := s.mail.SendPasswordReset(user.Email, resetURL); err != nil {
			s.log.Error("Failed to send reset password email",
				zap.String("email", user.Email), zap.Error(err))
		}
	})

	return nil
}

func (s *authService) ConfirmPasswordReset(ctx context.Context, token, newPassword string) error {
	tokenHash := utils.HashSHA256(token)
	prt, err := s.resetRepo.FindByTokenHash(ctx, tokenHash)
	if err != nil {
		return apperror.ErrInvalidToken
	}

	if time.Now().After(prt.ExpiresAt) {
		return apperror.ErrInvalidToken
	}

	// ищем пользователя
	user, err := s.userRepo.FindByID(ctx, prt.UserID)
	if err != nil {
		return err
	}

	// проверяем старый пароль и хешируем новый
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(newPassword)) == nil {
		return apperror.ErrSamePassword
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err = s.userRepo.UpdatePasswordHashTx(ctx, tx, user.ID, string(newHash)); err != nil {
		return fmt.Errorf("update password hash: %w", err)
	}

	if err = s.resetRepo.DeleteByUserIDTx(ctx, tx, user.ID); err != nil {
		return fmt.Errorf("mark token used: %w", err)
	}

	revokedIDs, err := s.tokenRepo.RevokeAllByUserTx(ctx, tx, user.ID, nil)
	if err != nil {
		return fmt.Errorf("revoke all tokens on password reset: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	// Инвалидация сессий в Redis и Kafka
	for _, sid := range revokedIDs {
		_ = s.rdb.Del(ctx, SessionKeyPrefix+sid.String()).Err()
		if s.producer != nil {
			evMsg := events.NewEvent(events.TypeSessionRevoked, events.SessionRevokedPayload{
				SessionID: sid,
				UserID:    user.ID,
				Reason:    "password_reset",
			})
			_ = s.producer.Publish(ctx, events.TopicAuthEvents, user.ID.String(), evMsg)
		}
	}

	return nil
}

func (s *authService) ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)) != nil {
		return apperror.ErrWrongPassword
	}

	if currentPassword == newPassword {
		return apperror.ErrSamePassword
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err = s.userRepo.UpdatePasswordHashTx(ctx, tx, user.ID, string(newHash)); err != nil {
		return fmt.Errorf("update password hash: %w", err)
	}

	revokedIDs, err := s.tokenRepo.RevokeAllByUserTx(ctx, tx, user.ID, nil)
	if err != nil {
		return fmt.Errorf("revoke other tokens on password change: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	// Инвалидируем остальные сессии
	for _, sid := range revokedIDs {
		_ = s.rdb.Del(ctx, SessionKeyPrefix+sid.String()).Err()
		if s.producer != nil {
			evMsg := events.NewEvent(events.TypeSessionRevoked, events.SessionRevokedPayload{
				SessionID: sid,
				UserID:    user.ID,
				Reason:    "password_changed",
			})
			_ = s.producer.Publish(ctx, events.TopicAuthEvents, user.ID.String(), evMsg)
		}
	}

	return nil
}

func (s *authService) FindByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return s.userRepo.FindByID(ctx, id)
}
