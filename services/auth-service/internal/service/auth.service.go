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
)

const (
	totpPendingKeyPrefix = "auth:totp:pending:"
	totpPendingTTL       = 10 * time.Minute
	totpIssuer           = "Chavo"
	backupCodesCount     = 8
)

type AuthService interface {
	Register(ctx context.Context, email, password string) error
	VerifyRegister(ctx context.Context, token string) error
	Login(ctx context.Context, email, password string) (challengeID, method string, err error)
	RequestPasswordReset(ctx context.Context, email string) error
	ConfirmPasswordReset(ctx context.Context, token, newPassword string) error
	ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error
	FindByID(ctx context.Context, id uuid.UUID) (*models.User, error)

	GetTwoFactor(ctx context.Context, userID uuid.UUID) (method string, backupRemaining int, err error)
	SetupTOTP(ctx context.Context, userID uuid.UUID, password string) (secret, otpauthURL string, err error)
	ConfirmTOTP(ctx context.Context, userID uuid.UUID, code string) ([]string, error)
	SwitchTwoFactorMethod(ctx context.Context, userID uuid.UUID, method, password, code string) error
	RegenerateBackupCodes(ctx context.Context, userID uuid.UUID, password, code string) ([]string, error)
	DeleteAccount(ctx context.Context, userID uuid.UUID, password string, code *string) error
}

type authService struct {
	userRepo     repository.UserRepository
	verRepo      repository.VerificationRepository
	resetRepo    repository.PasswordResetRepository
	tokenRepo    repository.TokenRepository
	backupRepo   repository.BackupCodeRepository
	outboxRepo   repository.OutboxRepository
	otpSvc       OtpService
	pool         *pgxpool.Pool
	rdb          *redis.Client
	notifyOutbox func()
	mail         email.EmailSender
	cfg          *config.Config
	log          *zap.Logger
}

func NewAuthService(
	userRepo repository.UserRepository,
	verRepo repository.VerificationRepository,
	resetRepo repository.PasswordResetRepository,
	tokenRepo repository.TokenRepository,
	backupRepo repository.BackupCodeRepository,
	outboxRepo repository.OutboxRepository,
	otpSvc OtpService,
	pool *pgxpool.Pool,
	rdb *redis.Client,
	notifyOutbox func(),
	mail email.EmailSender,
	cfg *config.Config,
	log *zap.Logger,
) AuthService {
	return &authService{
		userRepo:     userRepo,
		verRepo:      verRepo,
		resetRepo:    resetRepo,
		tokenRepo:    tokenRepo,
		backupRepo:   backupRepo,
		outboxRepo:   outboxRepo,
		otpSvc:       otpSvc,
		pool:         pool,
		rdb:          rdb,
		notifyOutbox: notifyOutbox,
		mail:         mail,
		cfg:          cfg,
		log:          log,
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

func (s *authService) Login(ctx context.Context, email, password string) (string, string, error) {
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			return "", "", apperror.ErrInvalidCredentials
		}
		return "", "", err
	}

	if !user.EmailVerified {
		return "", "", apperror.ErrAccountNotVerified
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return "", "", apperror.ErrInvalidCredentials
	}

	method := models.TwoFactorEmail
	if user.TwoFactorMethod == models.TwoFactorTOTP && len(user.TOTPSecretEnc) > 0 {
		method = models.TwoFactorTOTP
	}

	challengeID, err := s.otpSvc.CreateChallenge(ctx, user.ID, user.Email, method)
	if err != nil {
		return "", "", fmt.Errorf("create otp challenge: %w", err)
	}

	return challengeID, method, nil
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
	if s.outboxRepo != nil {
		for _, sid := range revokedIDs {
			evMsg := events.NewEvent(events.TypeSessionRevoked, events.SessionRevokedPayload{
				SessionID: sid,
				UserID:    user.ID,
				Reason:    "password_reset",
			})
			if err = s.outboxRepo.InsertTx(ctx, tx, events.TopicAuthEvents, user.ID.String(), evMsg); err != nil {
				return fmt.Errorf("enqueue session.revoked on password reset: %w", err)
			}
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	// немедленный запуск публикации outbox и инвалидация сессий в Redis
	if s.notifyOutbox != nil && len(revokedIDs) > 0 {
		s.notifyOutbox()
	}
	if err = deleteRedisKeys(ctx, s.rdb, s.log, sessionRedisKeys(revokedIDs...)...); err != nil {
		return fmt.Errorf("delete revoked sessions from redis on password reset: %w", err)
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
	if s.outboxRepo != nil {
		for _, sid := range revokedIDs {
			evMsg := events.NewEvent(events.TypeSessionRevoked, events.SessionRevokedPayload{
				SessionID: sid,
				UserID:    user.ID,
				Reason:    "password_changed",
			})
			if err = s.outboxRepo.InsertTx(ctx, tx, events.TopicAuthEvents, user.ID.String(), evMsg); err != nil {
				return fmt.Errorf("enqueue session.revoked on password change: %w", err)
			}
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	// будим outbox-воркер и инвалидируем остальные сессии в Redis
	if s.notifyOutbox != nil && len(revokedIDs) > 0 {
		s.notifyOutbox()
	}
	if err = deleteRedisKeys(ctx, s.rdb, s.log, sessionRedisKeys(revokedIDs...)...); err != nil {
		return fmt.Errorf("delete revoked sessions from redis on password change: %w", err)
	}

	return nil
}

func (s *authService) FindByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return s.userRepo.FindByID(ctx, id)
}

func (s *authService) GetTwoFactor(ctx context.Context, userID uuid.UUID) (string, int, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return "", 0, err
	}
	method := user.TwoFactorMethod
	if method != models.TwoFactorTOTP || len(user.TOTPSecretEnc) == 0 {
		return models.TwoFactorEmail, 0, nil
	}
	remaining, err := s.backupRepo.CountRemaining(ctx, userID)
	if err != nil {
		return "", 0, fmt.Errorf("count backup codes: %w", err)
	}
	return models.TwoFactorTOTP, remaining, nil
}

func (s *authService) SetupTOTP(ctx context.Context, userID uuid.UUID, password string) (string, string, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return "", "", err
	}
	if user.TwoFactorMethod == models.TwoFactorTOTP {
		return "", "", apperror.ErrTOTPAlreadyEnabled
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return "", "", apperror.ErrWrongPassword
	}

	secret, err := utils.GenerateTOTPSecret()
	if err != nil {
		return "", "", err
	}
	enc, err := utils.EncryptAESGCM(s.cfg.TOTPEncryptionKey, []byte(secret))
	if err != nil {
		return "", "", fmt.Errorf("encrypt pending totp secret: %w", err)
	}
	if err = s.rdb.Set(ctx, totpPendingKeyPrefix+userID.String(), enc, totpPendingTTL).Err(); err != nil {
		return "", "", fmt.Errorf("save pending totp secret: %w", err)
	}
	return secret, utils.BuildOTPAuthURL(totpIssuer, user.Email, secret), nil
}

func (s *authService) ConfirmTOTP(ctx context.Context, userID uuid.UUID, code string) ([]string, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.TwoFactorMethod == models.TwoFactorTOTP {
		return nil, apperror.ErrTOTPAlreadyEnabled
	}
	key := totpPendingKeyPrefix + userID.String()
	enc, err := s.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, apperror.ErrTOTPNotPending
		}
		return nil, fmt.Errorf("get pending totp secret: %w", err)
	}
	secretBytes, err := utils.DecryptAESGCMWithFallback(s.cfg.TOTPEncryptionKey, s.cfg.TOTPEncryptionPrevKeys, enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt pending totp secret: %w", err)
	}
	step, ok := utils.ValidateTOTP(string(secretBytes), code, time.Now())
	if !ok {
		return nil, apperror.ErrInvalidOTP
	}

	plainCodes, err := utils.GenerateBackupCodes(backupCodesCount)
	if err != nil {
		return nil, err
	}
	hashes := make([]string, len(plainCodes))
	for i, c := range plainCodes {
		hashes[i] = utils.HashBackupCode(c)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err = s.userRepo.EnableTOTPTx(ctx, tx, userID, enc); err != nil {
		return nil, err
	}
	if err = s.backupRepo.ReplaceAllTx(ctx, tx, userID, hashes); err != nil {
		return nil, fmt.Errorf("save backup codes: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	_ = s.rdb.Del(ctx, key).Err()
	usedKey := fmt.Sprintf("%s%s:%d", TOTPUsedKeyPrefix, userID.String(), step)
	_ = s.rdb.SetNX(ctx, usedKey, "1", TOTPUsedTTL).Err()

	return plainCodes, nil
}

func (s *authService) SwitchTwoFactorMethod(ctx context.Context, userID uuid.UUID, method, password, code string) error {
	// 2FA обязательна: сюда переходим только для возврата с totp на email
	if method != models.TwoFactorEmail {
		return apperror.ErrIncorrectData
	}
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.TwoFactorMethod != models.TwoFactorTOTP {
		return apperror.ErrTOTPNotEnabled
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return apperror.ErrWrongPassword
	}
	valid, err := s.otpSvc.VerifyTOTPCode(ctx, user, code, true)
	if err != nil {
		return err
	}
	if !valid {
		return apperror.ErrInvalidOTP
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err = s.userRepo.SwitchToEmail2FATx(ctx, tx, userID); err != nil {
		return err
	}
	if err = s.backupRepo.DeleteByUserIDTx(ctx, tx, userID); err != nil {
		return fmt.Errorf("delete backup codes: %w", err)
	}
	return tx.Commit(ctx)
}

func (s *authService) RegenerateBackupCodes(ctx context.Context, userID uuid.UUID, password, code string) ([]string, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.TwoFactorMethod != models.TwoFactorTOTP {
		return nil, apperror.ErrTOTPNotEnabled
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return nil, apperror.ErrWrongPassword
	}
	// для перевыпуска резервных кодов нужен живой код из приложения
	valid, err := s.otpSvc.VerifyTOTPCode(ctx, user, code, false)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, apperror.ErrInvalidOTP
	}

	plainCodes, err := utils.GenerateBackupCodes(backupCodesCount)
	if err != nil {
		return nil, err
	}
	hashes := make([]string, len(plainCodes))
	for i, c := range plainCodes {
		hashes[i] = utils.HashBackupCode(c)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err = s.backupRepo.ReplaceAllTx(ctx, tx, userID, hashes); err != nil {
		return nil, fmt.Errorf("replace backup codes: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return plainCodes, nil
}

func (s *authService) DeleteAccount(ctx context.Context, userID uuid.UUID, password string, code *string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return apperror.ErrWrongPassword
	}
	if user.TwoFactorMethod == models.TwoFactorTOTP {
		if code == nil || *code == "" {
			return apperror.ErrInvalidOTP
		}
		valid, err := s.otpSvc.VerifyTOTPCode(ctx, user, *code, true)
		if err != nil {
			return err
		}
		if !valid {
			return apperror.ErrInvalidOTP
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err = s.userRepo.SoftDeleteTx(ctx, tx, user.ID); err != nil {
		return err
	}
	if err = s.backupRepo.DeleteByUserIDTx(ctx, tx, user.ID); err != nil {
		return fmt.Errorf("delete backup codes: %w", err)
	}
	if err = s.verRepo.DeleteByUserIDTx(ctx, tx, user.ID); err != nil {
		return fmt.Errorf("delete verifications: %w", err)
	}
	if err = s.resetRepo.DeleteByUserIDTx(ctx, tx, user.ID); err != nil {
		return fmt.Errorf("delete password resets: %w", err)
	}
	revokedIDs, err := s.tokenRepo.RevokeAllByUserTx(ctx, tx, user.ID, nil)
	if err != nil {
		return fmt.Errorf("revoke all tokens on account delete: %w", err)
	}
	if s.outboxRepo != nil {
		evMsg := events.NewEvent(events.TypeAccountDeleted, events.AccountDeletedPayload{
			UserID:     user.ID,
			SessionIDs: revokedIDs,
		})
		if err = s.outboxRepo.InsertTx(ctx, tx, events.TopicAuthEvents, user.ID.String(), evMsg); err != nil {
			return fmt.Errorf("enqueue account_deleted event: %w", err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	if s.notifyOutbox != nil {
		s.notifyOutbox()
	}
	keys := make([]string, 0, len(revokedIDs)+1)
	keys = append(keys, totpPendingKeyPrefix+user.ID.String())
	keys = append(keys, sessionRedisKeys(revokedIDs...)...)
	if err = deleteRedisKeys(ctx, s.rdb, s.log, keys...); err != nil {
		return fmt.Errorf("delete revoked sessions from redis on account delete: %w", err)
	}

	return nil
}
