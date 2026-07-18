package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/auth-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/email"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/utils"
)

type OtpService interface {
	SendOTP(ctx context.Context, userID uuid.UUID, email string, codeType string) error
	VerifyAndMark(ctx context.Context, userID uuid.UUID, code, codeType string) (*models.OTPCode, error)
}

type otpService struct {
	repo repository.OTPRepository
	mail email.EmailSender
	log  *zap.Logger
}

func NewOtpService(repo repository.OTPRepository, mail email.EmailSender, log *zap.Logger) OtpService {
	return &otpService{repo: repo, mail: mail, log: log}
}

// SendOTP generates a 6-digit OTP, invalidates previous codes, saves, and sends via email.
func (s *otpService) SendOTP(ctx context.Context, userID uuid.UUID, email string, codeType string) error {
	code, err := utils.GenerateOTP()
	if err != nil {
		return fmt.Errorf("generate OTP: %w", err)
	}

	// Invalidate all previous OTPs for this user.
	if err = s.repo.InvalidateAll(ctx, userID); err != nil {
		return fmt.Errorf("invalidate previous OTPs: %w", err)
	}

	expiresAt := time.Now().Add(10 * time.Minute)
	otp := &models.OTPCode{
		UserID:    userID,
		Code:      code,
		CodeType:  codeType,
		ExpiresAt: expiresAt,
	}

	if err = s.repo.Create(ctx, otp); err != nil {
		return fmt.Errorf("save OTP: %w", err)
	}

	// Send OTP email in background.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.log.Error("Panic in send OTP goroutine", zap.Any("recover", r))
			}
		}()

		if err := s.mail.SendOTP(email, code, expiresAt.Format("15:04:05")); err != nil {
			s.log.Error("Failed to send OTP email",
				zap.String("email", email), zap.Error(err))
		}
	}()

	return nil
}

// VerifyAndMark finds a valid OTP for the user, verifies the code, and marks it as used.
func (s *otpService) VerifyAndMark(ctx context.Context, userID uuid.UUID, code string, codeType string) (*models.OTPCode, error) {
	otp, err := s.repo.FindValid(ctx, userID, code)
	if err != nil {
		return nil, apperror.ErrInvalidOTP
	}

	if otp.CodeType != codeType {
		return nil, apperror.ErrOTPTypeMismatch
	}

	if err = s.repo.MarkUsed(ctx, otp.ID); err != nil {
		return nil, fmt.Errorf("mark OTP as used: %w", err)
	}

	return otp, nil
}
