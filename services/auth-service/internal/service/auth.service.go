package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maltira/chavo-project-backend/services/auth-service/config"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/maltira/chavo-project-backend/services/auth-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/email"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/utils"
)

type AuthService interface {
	Register(ctx context.Context, email, password string) error
	VerifyNewAccount(ctx context.Context, token string) error
	Login(ctx context.Context, email, password string) (uuid.UUID, error)
	FindByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	ForgotPassword(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, tokenHash, newPassword string) error
	ValidatePasswordChange(ctx context.Context, userID uuid.UUID, currentPassword string) (*models.User, error)
	ApplyPasswordChange(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error
	ValidateEmailChange(ctx context.Context, userID uuid.UUID, newEmail string) (*models.User, error)
	ApplyEmailChange(ctx context.Context, userID uuid.UUID, newEmail string) error
	SoftDeleteUser(ctx context.Context, id uuid.UUID, email, reason, deletedBy string) error
}

type authService struct {
	userRepo  repository.UserRepository
	tokenRepo repository.TokenRepository
	pool      *pgxpool.Pool
	rdb       *redis.Client
	mail      email.EmailSender
	cfg       *config.Config
	log       *zap.Logger
}

func NewAuthService(
	userRepo repository.UserRepository,
	tokenRepo repository.TokenRepository,
	pool *pgxpool.Pool,
	rdb *redis.Client,
	mail email.EmailSender,
	cfg *config.Config,
	log *zap.Logger,
) AuthService {
	return &authService{
		userRepo:  userRepo,
		tokenRepo: tokenRepo,
		pool:      pool,
		rdb:       rdb,
		mail:      mail,
		cfg:       cfg,
		log:       log,
	}
}

// Register creates a new user and sends a verification email.
func (s *authService) Register(ctx context.Context, email, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	user, err := s.userRepo.Create(ctx, email, string(hash))
	if err != nil {
		return err // ErrEmailExists (already verified) or DB error
	}

	// Invalidate any previous verification token for this user
	// (covers the upsert case: user re-registers before confirming).
	refKey := "verify:ref:" + user.ID.String()
	if oldToken, rErr := s.rdb.Get(ctx, refKey).Result(); rErr == nil {
		s.rdb.Del(ctx, "verify:"+oldToken)
	}

	verificationToken := uuid.NewString()
	ttl := s.cfg.VerificationTTL

	if err = s.rdb.Set(ctx, "verify:"+verificationToken, user.ID.String(), ttl).Err(); err != nil {
		return fmt.Errorf("save verification token: %w", err)
	}
	// Reverse ref: user_id → token (for future invalidation).
	_ = s.rdb.Set(ctx, refKey, verificationToken, ttl).Err()

	// Send verification email in background.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.log.Error("Panic in verification email goroutine", zap.Any("recover", r))
			}
		}()

		verifyURL := fmt.Sprintf("%s/verify-email?token=%s", s.cfg.FrontendURL, verificationToken)
		if err = s.mail.SendVerification(email, verifyURL); err != nil {
			s.log.Error("Failed to send verification email",
				zap.String("email", email), zap.Error(err))
		}
	}()

	return nil
}

// VerifyNewAccount activates a user account using the verification token.
func (s *authService) VerifyNewAccount(ctx context.Context, token string) error {
	key := "verify:" + token
	userIDStr, err := s.rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return apperror.ErrInvalidToken
		}
		return fmt.Errorf("redis get: %w", err)
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return fmt.Errorf("parse user ID: %w", err)
	}

	user, err := s.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("find by id: %w", err)
	}
	if user.IsVerified {
		return nil // already verified — idempotent
	}

	// Begin transaction: SetVerified + event must succeed together.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err = s.userRepo.SetVerifiedTx(ctx, tx, userID); err != nil {
		return fmt.Errorf("set verified: %w", err)
	}

	// Notify user-service to create a profile (with retries).
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		err = utils.SendRequestCreateProfile(userID, s.cfg.UserServiceURL, s.cfg.InternalSecret)
		if err == nil {
			lastErr = nil
			break
		}

		lastErr = err
		s.log.Warn("Failed to send create profile request",
			zap.Int("attempt", attempt), zap.Error(err))

		if attempt < 3 {
			delay := time.Duration(attempt*300) * time.Millisecond
			time.Sleep(delay)
		}
	}

	if lastErr != nil {
		return fmt.Errorf("send request create profile: %w", lastErr)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	// Clean up both the token and its reverse ref.
	s.rdb.Del(ctx, key, "verify:ref:"+userID.String())

	s.log.Info("Account verified, profile created",
		zap.String("user_id", userID.String()))

	return nil
}

// Login authenticates a user by email and password.
func (s *authService) Login(ctx context.Context, email, password string) (uuid.UUID, error) {
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			return uuid.Nil, apperror.ErrInvalidCredentials
		}
		return uuid.Nil, err
	}

	if !user.IsVerified {
		return uuid.Nil, apperror.ErrAccountNotVerified
	}

	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		return uuid.Nil, apperror.ErrInvalidCredentials
	}

	return user.ID, nil
}

// FindByID returns a user by ID.
func (s *authService) FindByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return s.userRepo.FindByID(ctx, id)
}

// ForgotPassword generates a password reset token and sends it via email.
func (s *authService) ForgotPassword(ctx context.Context, email string) error {
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		// Don't reveal whether the email exists — always return success.
		if errors.Is(err, apperror.ErrNotFound) {
			return nil
		}
		return err
	}

	// Invalidate any existing reset token for this user.
	refKey := "reset:password:ref:" + user.ID.String()
	if oldHash, err := s.rdb.Get(ctx, refKey).Result(); err == nil {
		s.rdb.Del(ctx, "reset:password:"+oldHash)
	}
	s.rdb.Del(ctx, refKey)

	plainToken, err := utils.GenerateSecureToken(32)
	if err != nil {
		return fmt.Errorf("generate reset token: %w", err)
	}

	tokenHash := utils.HashSHA256(plainToken)
	ttl := s.cfg.VerificationTTL

	// Store: SHA256(token) → user_id (O(1) lookup during reset).
	if err = s.rdb.Set(ctx, "reset:password:"+tokenHash, user.ID.String(), ttl).Err(); err != nil {
		return fmt.Errorf("save reset token: %w", err)
	}
	// Reverse ref: user_id → hash (for invalidating previous tokens).
	_ = s.rdb.Set(ctx, refKey, tokenHash, ttl).Err()

	// Send reset email in background.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.log.Error("Panic in reset email goroutine", zap.Any("recover", r))
			}
		}()

		resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.cfg.FrontendURL, plainToken)
		if err = s.mail.SendPasswordReset(email, resetURL, ttl.String()); err != nil {
			s.log.Error("Failed to send reset password email",
				zap.String("email", email), zap.Error(err))
		}
	}()

	return nil
}

// ResetPassword changes the password using a hashed reset token.
func (s *authService) ResetPassword(ctx context.Context, tokenHash, newPassword string) error {
	resetKey := "reset:password:" + tokenHash
	userIDStr, err := s.rdb.Get(ctx, resetKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return apperror.ErrInvalidToken
		}
		return fmt.Errorf("redis get: %w", err)
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return fmt.Errorf("parse user ID: %w", err)
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	user.Password = string(hash)
	user.PasswordUpdatedAt = time.Now()
	if err = s.userRepo.Update(ctx, user); err != nil {
		return err
	}

	// Clean up used tokens.
	s.rdb.Del(ctx, resetKey)
	s.rdb.Del(ctx, "reset:password:ref:"+userID.String())

	// Revoke all refresh tokens — force re-login on all devices.
	jtis, err := s.tokenRepo.DeleteAllByUser(ctx, userID, nil)
	if err != nil {
		s.log.Warn("Failed to revoke tokens after password reset", zap.Error(err))
	}
	s.blacklistJTIs(ctx, jtis)

	return nil
}

// ValidatePasswordChange checks that the current password is correct.
func (s *authService) ValidatePasswordChange(ctx context.Context, userID uuid.UUID, currentPassword string) (*models.User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(currentPassword)) != nil {
		return nil, apperror.ErrWrongPassword
	}

	return user, nil
}

// ApplyPasswordChange hashes and sets a new password, then revokes all sessions.
func (s *authService) ApplyPasswordChange(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error {
	user, err := s.ValidatePasswordChange(ctx, userID, currentPassword)
	if err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	user.Password = string(hash)
	user.PasswordUpdatedAt = time.Now()
	if err = s.userRepo.Update(ctx, user); err != nil {
		return err
	}

	jtis, err := s.tokenRepo.DeleteAllByUser(ctx, userID, nil)
	if err != nil {
		s.log.Warn("Failed to revoke tokens after password change", zap.Error(err))
	}
	s.blacklistJTIs(ctx, jtis)

	return nil
}

// ValidateEmailChange checks that the new email is different and not taken.
func (s *authService) ValidateEmailChange(ctx context.Context, userID uuid.UUID, newEmail string) (*models.User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if user.Email == newEmail {
		return nil, apperror.ErrSameEmail
	}

	_, err = s.userRepo.FindByEmail(ctx, newEmail)
	if err == nil {
		return nil, apperror.ErrEmailExists
	}
	if !errors.Is(err, apperror.ErrNotFound) {
		return nil, err
	}

	return user, nil
}

// ApplyEmailChange updates the user's email.
func (s *authService) ApplyEmailChange(ctx context.Context, userID uuid.UUID, newEmail string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	user.Email = newEmail
	user.EmailUpdatedAt = time.Now()
	return s.userRepo.Update(ctx, user)
}

// SoftDeleteUser performs a soft delete of the user.
func (s *authService) SoftDeleteUser(ctx context.Context, id uuid.UUID, email, reason, deletedBy string) error {
	return s.userRepo.SoftDelete(ctx, id, email, reason, deletedBy)
}

// blacklistJTIs adds access token JTIs to the Redis blacklist.
func (s *authService) blacklistJTIs(ctx context.Context, jtis []string) {
	for _, jti := range jtis {
		key := "blacklist:jti:" + jti
		if err := s.rdb.Set(ctx, key, "1", s.cfg.AccessTokenDuration).Err(); err != nil {
			s.log.Warn("Failed to blacklist JTI", zap.String("jti", jti), zap.Error(err))
		}
	}
}
