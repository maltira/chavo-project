package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maltira/chavo-project-backend/services/auth-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/models"
)

type VerificationRepository interface {
	Create(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error
	FindByTokenHash(ctx context.Context, tokenHash string) (*models.EmailVerification, error)
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	DeleteByUserIDTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error
}

type verificationRepository struct {
	pool *pgxpool.Pool
}

func NewVerificationRepository(pool *pgxpool.Pool) VerificationRepository {
	return &verificationRepository{pool: pool}
}

func (r *verificationRepository) Create(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO email_verifications (user_id, token_hash, expires_at, created_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (user_id) DO UPDATE
		   SET token_hash = EXCLUDED.token_hash,
		       expires_at = EXCLUDED.expires_at,
		       created_at = NOW()`,
		userID, tokenHash, expiresAt,
	)
	return err
}

func (r *verificationRepository) FindByTokenHash(ctx context.Context, tokenHash string) (*models.EmailVerification, error) {
	var ev models.EmailVerification
	err := r.pool.QueryRow(ctx,
		`SELECT user_id, token_hash, expires_at, created_at
		 FROM email_verifications
		 WHERE token_hash = $1`, tokenHash,
	).Scan(&ev.UserID, &ev.TokenHash, &ev.ExpiresAt, &ev.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrInvalidToken
		}
		return nil, err
	}
	return &ev, nil
}

func (r *verificationRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM email_verifications WHERE user_id = $1`, userID)
	return err
}

func (r *verificationRepository) DeleteByUserIDTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	_, err := tx.Exec(ctx, `DELETE FROM email_verifications WHERE user_id = $1`, userID)
	return err
}
