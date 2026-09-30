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

type PasswordResetRepository interface {
	Create(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error
	FindByTokenHash(ctx context.Context, tokenHash string) (*models.PasswordResetToken, error)
	DeleteByUserIDTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error
}

type passwordResetRepository struct {
	pool *pgxpool.Pool
}

func NewPasswordResetRepository(pool *pgxpool.Pool) PasswordResetRepository {
	return &passwordResetRepository{pool: pool}
}

func (r *passwordResetRepository) Create(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO password_reset_tokens (user_id, token_hash, expires_at, created_at)
		 VALUES ($1, $2, $3, NOW())`,
		userID, tokenHash, expiresAt,
	)
	return err
}

func (r *passwordResetRepository) FindByTokenHash(ctx context.Context, tokenHash string) (*models.PasswordResetToken, error) {
	var prt models.PasswordResetToken
	err := r.pool.QueryRow(ctx,
		`SELECT user_id, token_hash, expires_at, created_at
		 FROM password_reset_tokens WHERE token_hash = $1`, tokenHash,
	).Scan(&prt.UserID, &prt.TokenHash, &prt.ExpiresAt, &prt.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrInvalidToken
		}
		return nil, err
	}
	return &prt, nil
}

func (r *passwordResetRepository) DeleteByUserIDTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	_, err := tx.Exec(ctx, `DELETE FROM password_reset_tokens WHERE user_id = $1`, userID)
	return err
}
