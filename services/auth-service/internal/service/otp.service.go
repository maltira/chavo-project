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
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/utils"
)

const (
	OTPChallengeTTL = 5 * time.Minute
	MaxOTPAttempts  = 5
	OTPKeyPrefix    = "auth:otp:challenge:"
)

type otpChallengeData struct {
	UserID   uuid.UUID `json:"user_id"`
	OTPHash  string    `json:"otp_hash"`
	Attempts int       `json:"attempts"`
}

type OtpService interface {
	CreateChallenge(ctx context.Context, userID uuid.UUID, email string) (challengeID string, err error)
	VerifyChallenge(ctx context.Context, challengeID string, code string) (userID uuid.UUID, err error)
	ResendChallenge(ctx context.Context, challengeID string) error
}

type otpService struct {
	rdb      *redis.Client
	userRepo repository.UserRepository
	mail     email.EmailSender
	log      *zap.Logger
}

func NewOtpService(
	rdb *redis.Client,
	userRepo repository.UserRepository,
	mail email.EmailSender,
	log *zap.Logger,
) OtpService {
	return &otpService{
		rdb:      rdb,
		userRepo: userRepo,
		mail:     mail,
		log:      log,
	}
}

func (s *otpService) CreateChallenge(ctx context.Context, userID uuid.UUID, toEmail string) (string, error) {
	code, err := utils.GenerateOTP()
	if err != nil {
		return "", fmt.Errorf("generate otp: %w", err)
	}

	challengeID := uuid.NewString()
	data := otpChallengeData{
		UserID:   userID,
		OTPHash:  utils.HashSHA256(code),
		Attempts: 0,
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal otp challenge: %w", err)
	}

	key := OTPKeyPrefix + challengeID
	if err = s.rdb.Set(ctx, key, payload, OTPChallengeTTL).Err(); err != nil {
		return "", fmt.Errorf("save otp challenge to redis: %w", err)
	}

	// Отправляем email в фоне
	utils.SafeGo(s.log, "send OTP email", func() {
		if err := s.mail.SendOTP(toEmail, code, "5 минут"); err != nil {
			s.log.Error("Failed to send OTP email", zap.String("email", toEmail), zap.Error(err))
		}
	})

	return challengeID, nil
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

	inputHash := utils.HashSHA256(code)
	if inputHash != data.OTPHash {
		data.Attempts++
		if data.Attempts >= MaxOTPAttempts {
			_ = s.rdb.Del(ctx, key).Err()
			return uuid.Nil, apperror.ErrOTPAttemptsExceeded
		}

		// Обновляем количество попыток, сохраняя оставшийся TTL
		ttl, _ := s.rdb.TTL(ctx, key).Result()
		if ttl > 0 {
			if updatedPayload, err := json.Marshal(data); err == nil {
				_ = s.rdb.Set(ctx, key, updatedPayload, ttl).Err()
			}
		}

		return uuid.Nil, apperror.ErrInvalidOTP
	}

	// Успешная верификация — удаляем challenge
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
