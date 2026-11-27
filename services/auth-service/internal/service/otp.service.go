package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/auth-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/email"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/utils"
)

const (
	OTPChallengeTTL   = 5 * time.Minute
	MaxOTPAttempts    = 5
	OTPKeyPrefix      = "auth:otp:challenge:"
	TOTPUsedKeyPrefix = "auth:totp:used:"
	TOTPUsedTTL       = 90 * time.Second
)

type otpChallengeData struct {
	UserID   uuid.UUID `json:"user_id"`
	Method   string    `json:"method"`
	OTPHash  string    `json:"otp_hash,omitempty"`
	Attempts int       `json:"attempts"`
}

type OtpService interface {
	CreateChallenge(ctx context.Context, userID uuid.UUID, email, method string) (challengeID string, err error)
	VerifyChallenge(ctx context.Context, challengeID string, code string) (userID uuid.UUID, err error)
	ResendChallenge(ctx context.Context, challengeID string) error
	// VerifyTOTPCode проверяет 6-значный TOTP-код с защитой от повтора шага (и опционально резервный код)
	VerifyTOTPCode(ctx context.Context, user *models.User, code string, allowBackup bool) (bool, error)
}

type otpService struct {
	rdb        *redis.Client
	userRepo   repository.UserRepository
	backupRepo repository.BackupCodeRepository
	totpKey    []byte
	mail       email.EmailSender
	log        *zap.Logger
}

func NewOtpService(
	rdb *redis.Client,
	userRepo repository.UserRepository,
	backupRepo repository.BackupCodeRepository,
	totpKey []byte,
	mail email.EmailSender,
	log *zap.Logger,
) OtpService {
	return &otpService{
		rdb:        rdb,
		userRepo:   userRepo,
		backupRepo: backupRepo,
		totpKey:    totpKey,
		mail:       mail,
		log:        log,
	}
}

func (s *otpService) CreateChallenge(ctx context.Context, userID uuid.UUID, toEmail, method string) (string, error) {
	if method != models.TwoFactorTOTP {
		method = models.TwoFactorEmail
	}

	challengeID := uuid.NewString()
	data := otpChallengeData{
		UserID:   userID,
		Method:   method,
		Attempts: 0,
	}

	var emailCode string
	if method == models.TwoFactorEmail {
		code, err := utils.GenerateOTP()
		if err != nil {
			return "", fmt.Errorf("generate otp: %w", err)
		}
		emailCode = code
		data.OTPHash = utils.HashSHA256(code)
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal otp challenge: %w", err)
	}

	key := OTPKeyPrefix + challengeID
	if err = s.rdb.Set(ctx, key, payload, OTPChallengeTTL).Err(); err != nil {
		return "", fmt.Errorf("save otp challenge to redis: %w", err)
	}

	if method == models.TwoFactorEmail {
		utils.SafeGo(s.log, "send OTP email", func() {
			if err := s.mail.SendOTP(toEmail, emailCode, "5 минут"); err != nil {
				s.log.Error("Failed to send OTP email", zap.String("email", toEmail), zap.Error(err))
			}
		})
	}

	return challengeID, nil
}

func (s *otpService) VerifyTOTPCode(ctx context.Context, user *models.User, code string, allowBackup bool) (bool, error) {
	// 8-символьный резервный код проверяем до расшифровки TOTP-секрета: он не пересекается с 6-значным TOTP
	// и остаётся рабочим даже при смене ключа шифрования
	if allowBackup && s.backupRepo != nil && len(utils.NormalizeBackupCode(code)) == 8 {
		return s.backupRepo.Consume(ctx, user.ID, utils.HashBackupCode(code))
	}
	if len(user.TOTPSecretEnc) == 0 {
		return false, nil
	}
	secretBytes, err := utils.DecryptAESGCM(s.totpKey, user.TOTPSecretEnc)
	if err != nil {
		return false, fmt.Errorf("decrypt totp secret: %w", err)
	}
	if step, ok := utils.ValidateTOTP(string(secretBytes), code, time.Now()); ok {
		// один временной шаг нельзя использовать повторно
		usedKey := fmt.Sprintf("%s%s:%d", TOTPUsedKeyPrefix, user.ID.String(), step)
		claimed, err := s.rdb.SetNX(ctx, usedKey, "1", TOTPUsedTTL).Result()
		if err != nil {
			return false, fmt.Errorf("mark totp step used: %w", err)
		}
		return claimed, nil
	}
	return false, nil
}

func (s *otpService) VerifyChallenge(ctx context.Context, challengeID string, code string) (uuid.UUID, error) {
	key := OTPKeyPrefix + challengeID
	raw, err := s.rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return uuid.Nil, apperror.ErrInvalidChallenge
		}
		return uuid.Nil, fmt.Errorf("redis get otp challenge: %w", err)
	}

	var data otpChallengeData
	if err = json.Unmarshal([]byte(raw), &data); err != nil {
		return uuid.Nil, fmt.Errorf("unmarshal otp challenge: %w", err)
	}

	if data.Attempts >= MaxOTPAttempts {
		_ = s.rdb.Del(ctx, key).Err()
		return uuid.Nil, apperror.ErrOTPAttemptsExceeded
	}

	// проверяем активность пользователя для обоих методов (email и totp), чтобы удалённому аккаунту не выдать сессию
	user, err := s.userRepo.FindByID(ctx, data.UserID)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			_ = s.rdb.Del(ctx, key).Err()
			return uuid.Nil, apperror.ErrInvalidChallenge
		}
		return uuid.Nil, fmt.Errorf("find user for otp verify: %w", err)
	}

	var valid bool
	if data.Method == models.TwoFactorTOTP {
		valid, err = s.VerifyTOTPCode(ctx, user, code, true)
		if err != nil {
			return uuid.Nil, err
		}
	} else {
		valid = utils.HashSHA256(code) == data.OTPHash
	}

	if !valid {
		data.Attempts++
		if data.Attempts >= MaxOTPAttempts {
			_ = s.rdb.Del(ctx, key).Err()
			return uuid.Nil, apperror.ErrOTPAttemptsExceeded
		}

		// обновляем количество попыток, сохраняя оставшийся TTL
		ttl, _ := s.rdb.TTL(ctx, key).Result()
		if ttl > 0 {
			if updatedPayload, err := json.Marshal(data); err == nil {
				_ = s.rdb.Set(ctx, key, updatedPayload, ttl).Err()
			}
		}

		return uuid.Nil, apperror.ErrInvalidOTP
	}

	// успешная верификация - удаляем challenge
	_ = s.rdb.Del(ctx, key).Err()

	return data.UserID, nil
}

func (s *otpService) ResendChallenge(ctx context.Context, challengeID string) error {
	key := OTPKeyPrefix + challengeID
	raw, err := s.rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return apperror.ErrInvalidChallenge
		}
		return fmt.Errorf("redis get otp challenge: %w", err)
	}

	var data otpChallengeData
	if err = json.Unmarshal([]byte(raw), &data); err != nil {
		return fmt.Errorf("unmarshal otp challenge: %w", err)
	}

	if data.Method == models.TwoFactorTOTP {
		return apperror.ErrResendNotAllowed
	}

	user, err := s.userRepo.FindByID(ctx, data.UserID)
	if err != nil {
		return fmt.Errorf("find user for resend otp: %w", err)
	}

	code, err := utils.GenerateOTP()
	if err != nil {
		return fmt.Errorf("generate otp: %w", err)
	}

	data.OTPHash = utils.HashSHA256(code)
	data.Attempts = 0

	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal otp challenge: %w", err)
	}

	if err = s.rdb.Set(ctx, key, payload, OTPChallengeTTL).Err(); err != nil {
		return fmt.Errorf("save updated otp challenge to redis: %w", err)
	}

	utils.SafeGo(s.log, "resend OTP email", func() {
		if err := s.mail.SendOTP(user.Email, code, "5 минут"); err != nil {
			s.log.Error("Failed to send OTP email", zap.String("email", user.Email), zap.Error(err))
		}
	})

	return nil
}
