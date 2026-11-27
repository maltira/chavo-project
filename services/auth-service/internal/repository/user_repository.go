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
	EnableTOTPTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, secretEnc []byte) error
	SwitchToEmail2FATx(ctx context.Context, tx pgx.Tx, id uuid.UUID) error
	SoftDeleteTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) error
}

type userRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &userRepository{pool: pool}
}

const userColumns = `id, email, password_hash, email_verified, two_factor_method, totp_secret_enc, totp_verified_at, created_at, updated_at, deleted_at`

func (r *userRepository) CreateOrUpdateUnverified(ctx context.Context, email, passwordHash string) (*models.User, error) {
	var user models.User
	err := r.pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash)
		 VALUES ($1, $2)
		 ON CONFLICT (email) WHERE deleted_at IS NULL DO UPDATE
		   SET password_hash = EXCLUDED.password_hash,
		       updated_at = NOW()
		   WHERE users.email_verified = FALSE
		 RETURNING `+userColumns,
		email, passwordHash,
	).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.EmailVerified,
		&user.TwoFactorMethod, &user.TOTPSecretEnc, &user.TOTPVerifiedAt,
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
		`SELECT `+userColumns+` FROM users WHERE id = $1 AND deleted_at IS NULL`, id)
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	return r.scanUser(ctx,
		`SELECT `+userColumns+` FROM users WHERE email = $1 AND deleted_at IS NULL`, email)
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

func (r *userRepository) EnableTOTPTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, secretEnc []byte) error {
	ct, err := tx.Exec(ctx,
		`UPDATE users
		 SET two_factor_method = 'totp', totp_secret_enc = $1, totp_verified_at = NOW(), updated_at = NOW()
		 WHERE id = $2 AND deleted_at IS NULL AND two_factor_method <> 'totp'`,
		secretEnc, id,
	)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return apperror.ErrTOTPAlreadyEnabled
	}
	return nil
}

func (r *userRepository) SwitchToEmail2FATx(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	ct, err := tx.Exec(ctx,
		`UPDATE users
		 SET two_factor_method = 'email', totp_secret_enc = NULL, totp_verified_at = NULL, updated_at = NOW()
		 WHERE id = $1 AND deleted_at IS NULL`,
		id,
	)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

func (r *userRepository) SoftDeleteTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	ct, err := tx.Exec(ctx,
		`UPDATE users
		 SET deleted_at = NOW(),
		     two_factor_method = 'email',
		     totp_secret_enc = NULL,
		     totp_verified_at = NULL,
		     updated_at = NOW()
		 WHERE id = $1 AND deleted_at IS NULL`,
		id,
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
		&user.TwoFactorMethod, &user.TOTPSecretEnc, &user.TOTPVerifiedAt,
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
