package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maltira/chavo-project-backend/services/auth-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/models"
)

type UserRepository interface {
	CreateOrUpdateUnverified(ctx context.Context, email, passwordHash string) (*models.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	SetVerifiedTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) error
	UpdatePasswordHashTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, newHash string) error
}

type userRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &userRepository{pool: pool}
}

func (r *userRepository) CreateOrUpdateUnverified(ctx context.Context, email, passwordHash string) (*models.User, error) {
	var user models.User
	err := r.pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash)
		 VALUES ($1, $2)
		 ON CONFLICT (email) DO UPDATE
		   SET password_hash = EXCLUDED.password_hash,
		       updated_at = NOW()
		   WHERE users.email_verified = FALSE AND users.deleted_at IS NULL
		 RETURNING id, email, password_hash, email_verified, created_at, updated_at, deleted_at`,
		email, passwordHash,
	).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.EmailVerified,
		&user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// DO UPDATE WHERE users.email_verified = FALSE не сработал, значит email уже подтвержден
			return nil, apperror.ErrEmailExists
		}
		if isDuplicateKey(err) {
			return nil, apperror.ErrEmailExists
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return r.scanUser(ctx,
		`SELECT id, email, password_hash, email_verified, created_at, updated_at, deleted_at
		 FROM users WHERE id = $1 AND deleted_at IS NULL`, id)
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	return r.scanUser(ctx,
		`SELECT id, email, password_hash, email_verified, created_at, updated_at, deleted_at
		 FROM users WHERE email = $1 AND deleted_at IS NULL`, email)
}

func (r *userRepository) SetVerifiedTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	ct, err := tx.Exec(ctx,
		`UPDATE users SET email_verified = TRUE, updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

func (r *userRepository) UpdatePasswordHashTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, newHash string) error {
	ct, err := tx.Exec(ctx,
		`UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL`,
		newHash, id,
	)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

func (r *userRepository) scanUser(ctx context.Context, query string, args ...any) (*models.User, error) {
	var user models.User
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.EmailVerified,
		&user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func isDuplicateKey(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
