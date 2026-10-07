package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maltira/chavo-project-backend/services/auth-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/models"
)

type TokenRepository interface {
	Save(ctx context.Context, token *models.RefreshToken) error
	FindByTokenHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error)
	FindByID(ctx context.Context, id uuid.UUID) (*models.RefreshToken, error)
	RevokeByID(ctx context.Context, id uuid.UUID) error
	// Rotate заменяет refresh-токен действующей сессии, если её хэш всё ещё oldHash; false — токен уже сменили или отозвали.
	Rotate(ctx context.Context, id uuid.UUID, oldHash, newHash string, expiresAt time.Time, ip, userAgent, device *string) (bool, error)
	RevokeAllByUserTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, excludeID *uuid.UUID) ([]uuid.UUID, error)
	ListActiveByUser(ctx context.Context, userID uuid.UUID) ([]models.RefreshToken, error)
}

type tokenRepository struct {
	pool *pgxpool.Pool
}

func NewTokenRepository(pool *pgxpool.Pool) TokenRepository {
	return &tokenRepository{pool: pool}
}

func (r *tokenRepository) Save(ctx context.Context, token *models.RefreshToken) error {
	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO refresh_tokens (id, user_id, token_hash, device_name, user_agent, ip_address, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())`,
		token.ID, token.UserID, token.TokenHash,
		token.DeviceName, token.UserAgent, token.IPAddress,
		token.ExpiresAt,
	)
	return err
}

func (r *tokenRepository) FindByTokenHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	return r.scanToken(ctx,
		`SELECT id, user_id, token_hash, device_name, user_agent, host(ip_address), expires_at, created_at, revoked_at
		 FROM refresh_tokens WHERE token_hash = $1`, tokenHash)
}

func (r *tokenRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.RefreshToken, error) {
	return r.scanToken(ctx,
		`SELECT id, user_id, token_hash, device_name, user_agent, host(ip_address), expires_at, created_at, revoked_at
		 FROM refresh_tokens WHERE id = $1`, id)
}

func (r *tokenRepository) RevokeByID(ctx context.Context, id uuid.UUID) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

func (r *tokenRepository) RevokeAllByUserTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, excludeID *uuid.UUID) ([]uuid.UUID, error) {
	query := `UPDATE refresh_tokens 
	          SET revoked_at = NOW() 
	          WHERE user_id = $1 AND revoked_at IS NULL`
	args := []any{userID}

	if excludeID != nil && *excludeID != uuid.Nil {
		query += ` AND id != $2`
		args = append(args, *excludeID)
	}
	query += ` RETURNING id`

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessionIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		sessionIDs = append(sessionIDs, id)
	}
	return sessionIDs, rows.Err()
}

func (r *tokenRepository) ListActiveByUser(ctx context.Context, userID uuid.UUID) ([]models.RefreshToken, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, token_hash, device_name, user_agent, host(ip_address), expires_at, created_at, revoked_at
		 FROM refresh_tokens
		 WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > NOW()
		 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []models.RefreshToken
	for rows.Next() {
		var t models.RefreshToken
		if err = rows.Scan(
			&t.ID, &t.UserID, &t.TokenHash,
			&t.DeviceName, &t.UserAgent, &t.IPAddress,
			&t.ExpiresAt, &t.CreatedAt, &t.RevokedAt,
		); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

func (r *tokenRepository) scanToken(ctx context.Context, query string, args ...any) (*models.RefreshToken, error) {
	var t models.RefreshToken
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&t.ID, &t.UserID, &t.TokenHash,
		&t.DeviceName, &t.UserAgent, &t.IPAddress,
		&t.ExpiresAt, &t.CreatedAt, &t.RevokedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *tokenRepository) Rotate(ctx context.Context, id uuid.UUID, oldHash, newHash string, expiresAt time.Time, ip, userAgent, device *string) (bool, error) {
	ct, err := r.pool.Exec(ctx,
		`UPDATE refresh_tokens
		 SET token_hash = $3, expires_at = $4,
		     ip_address = COALESCE($5::inet, ip_address),
		     user_agent = COALESCE($6, user_agent),
		     device_name = COALESCE($7, device_name)
		 WHERE id = $1 AND token_hash = $2 AND revoked_at IS NULL`,
		id, oldHash, newHash, expiresAt, ip, userAgent, device)
	if err != nil {
		return false, fmt.Errorf("rotate refresh token: %w", err)
	}
	return ct.RowsAffected() == 1, nil
}
