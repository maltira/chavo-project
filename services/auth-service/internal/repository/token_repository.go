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

type TokenRepository interface {
	Save(ctx context.Context, token *models.RefreshToken) error
	FindByToken(ctx context.Context, token string) (*models.RefreshToken, error)
	FindByID(ctx context.Context, id uuid.UUID) (*models.RefreshToken, error)
	Delete(ctx context.Context, token string) error
	DeleteAllByUser(ctx context.Context, userID uuid.UUID, excludeToken *string) ([]string, error)
	ListActiveByUser(ctx context.Context, userID uuid.UUID) ([]models.RefreshToken, error)
}

type tokenRepository struct {
	pool *pgxpool.Pool
}

func NewTokenRepository(pool *pgxpool.Pool) TokenRepository {
	return &tokenRepository{pool: pool}
}

func (r *tokenRepository) Save(ctx context.Context, token *models.RefreshToken) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token, access_jti, ip, user_agent, device, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		token.UserID, token.Token, token.AccessJTI,
		token.IP, token.UserAgent, token.Device,
		token.ExpiresAt,
	)
	return err
}

func (r *tokenRepository) FindByToken(ctx context.Context, token string) (*models.RefreshToken, error) {
	return r.scanToken(ctx,
		`SELECT id, user_id, token, access_jti, ip, user_agent, device, created_at, expires_at
		 FROM refresh_tokens WHERE token = $1`, token)
}

func (r *tokenRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.RefreshToken, error) {
	return r.scanToken(ctx,
		`SELECT id, user_id, token, access_jti, ip, user_agent, device, created_at, expires_at
		 FROM refresh_tokens WHERE id = $1`, id)
}

func (r *tokenRepository) Delete(ctx context.Context, token string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM refresh_tokens WHERE token = $1`, token)
	return err
}

// DeleteAllByUser deletes all refresh tokens for a user (optionally excluding one)
// and returns the access JTIs so they can be blacklisted.
func (r *tokenRepository) DeleteAllByUser(ctx context.Context, userID uuid.UUID, excludeToken *string) ([]string, error) {
	query := `DELETE FROM refresh_tokens WHERE user_id = $1`
	args := []any{userID}

	if excludeToken != nil && *excludeToken != "" {
		query += ` AND token != $2`
		args = append(args, *excludeToken)
	}

	query += ` RETURNING access_jti`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jtis []string
	for rows.Next() {
		var jti string
		if err = rows.Scan(&jti); err != nil {
			return nil, err
		}
		if jti != "" {
			jtis = append(jtis, jti)
		}
	}
	return jtis, rows.Err()
}

func (r *tokenRepository) ListActiveByUser(ctx context.Context, userID uuid.UUID) ([]models.RefreshToken, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, token, access_jti, ip, user_agent, device, created_at, expires_at
		 FROM refresh_tokens
		 WHERE user_id = $1 AND expires_at > NOW()
		 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []models.RefreshToken
	for rows.Next() {
		var t models.RefreshToken
		if err = rows.Scan(
			&t.ID, &t.UserID, &t.Token, &t.AccessJTI,
			&t.IP, &t.UserAgent, &t.Device,
			&t.CreatedAt, &t.ExpiresAt,
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
		&t.ID, &t.UserID, &t.Token, &t.AccessJTI,
		&t.IP, &t.UserAgent, &t.Device,
		&t.CreatedAt, &t.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}
