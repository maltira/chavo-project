package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maltira/chavo-project-backend/services/auth-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/models"
)

type OTPRepository interface {
	Create(ctx context.Context, otp *models.OTPCode) error
	FindValid(ctx context.Context, userID uuid.UUID, code string) (*models.OTPCode, error)
	MarkUsed(ctx context.Context, id uuid.UUID) error
	InvalidateAll(ctx context.Context, userID uuid.UUID) error
}

type otpRepository struct {
	pool *pgxpool.Pool
}

func NewOTPRepository(pool *pgxpool.Pool) OTPRepository {
	return &otpRepository{pool: pool}
}

func (r *otpRepository) Create(ctx context.Context, otp *models.OTPCode) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO otp_codes (user_id, code, code_type, expires_at)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		otp.UserID, otp.Code, otp.CodeType, otp.ExpiresAt,
	).Scan(&otp.ID, &otp.CreatedAt)
}

func (r *otpRepository) FindValid(ctx context.Context, userID uuid.UUID, code string) (*models.OTPCode, error) {
	var otp models.OTPCode
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, code, code_type, is_used, created_at, expires_at
		 FROM otp_codes
		 WHERE user_id = $1 AND code = $2 AND is_used = FALSE AND expires_at > NOW()
		 ORDER BY created_at DESC
		 LIMIT 1`,
		userID, code,
	).Scan(
		&otp.ID, &otp.UserID, &otp.Code, &otp.CodeType,
		&otp.IsUsed, &otp.CreatedAt, &otp.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrInvalidOTP
		}
		return nil, err
	}
	return &otp, nil
}

func (r *otpRepository) MarkUsed(ctx context.Context, id uuid.UUID) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE otp_codes SET is_used = TRUE WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

func (r *otpRepository) InvalidateAll(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE otp_codes SET is_used = TRUE WHERE user_id = $1 AND is_used = FALSE`, userID)
	return err
}
