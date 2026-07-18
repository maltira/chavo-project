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
	Create(ctx context.Context, email, hashedPassword string) (*models.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	SetVerified(ctx context.Context, id uuid.UUID) error
	SetVerifiedTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) error
	SoftDelete(ctx context.Context, id uuid.UUID, email, reason, deletedBy string) error
}

type userRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &userRepository{pool: pool}
}

func (r *userRepository) Create(ctx context.Context, email, hashedPassword string) (*models.User, error) {
	var user models.User
	err := r.pool.QueryRow(ctx,
		`INSERT INTO users (email, password)
		 VALUES ($1, $2)
		 ON CONFLICT (email) DO UPDATE
		   SET password = $2
		   WHERE users.is_verified = FALSE
		 RETURNING id, email, password, is_verified,
		           deleted_at, deletion_reason, deleted_by,
		           created_at, password_updated_at, email_updated_at`,
		email, hashedPassword,
	).Scan(
		&user.ID, &user.Email, &user.Password, &user.IsVerified,
		&user.DeletedAt, &user.DeletionReason, &user.DeletedBy,
		&user.CreatedAt, &user.PasswordUpdatedAt, &user.EmailUpdatedAt,
	)
	if err != nil {
		// pgx возвращает ErrNoRows когда DO UPDATE WHERE не сработал —
		// это означает, что пользователь с таким email уже верифицирован.
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrEmailExists
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return r.scanUser(ctx,
		`SELECT id, email, password, is_verified,
		        deleted_at, deletion_reason, deleted_by,
		        created_at, password_updated_at, email_updated_at
		 FROM users WHERE id = $1 AND deleted_at IS NULL`, id)
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	return r.scanUser(ctx,
		`SELECT id, email, password, is_verified,
		        deleted_at, deletion_reason, deleted_by,
		        created_at, password_updated_at, email_updated_at
		 FROM users WHERE email = $1 AND deleted_at IS NULL`, email)
}

func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE users
		 SET email = $1, password = $2, is_verified = $3,
		     password_updated_at = $4, email_updated_at = $5
		 WHERE id = $6 AND deleted_at IS NULL`,
		user.Email, user.Password, user.IsVerified,
		user.PasswordUpdatedAt, user.EmailUpdatedAt,
		user.ID,
	)
	if err != nil {
		if isDuplicateKey(err) {
			return apperror.ErrEmailExists
		}
		return err
	}
	if ct.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

func (r *userRepository) SetVerified(ctx context.Context, id uuid.UUID) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE users SET is_verified = TRUE WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

func (r *userRepository) SetVerifiedTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	ct, err := tx.Exec(ctx,
		`UPDATE users SET is_verified = TRUE WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

func (r *userRepository) SoftDelete(ctx context.Context, id uuid.UUID, email, reason, deletedBy string) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE users
		 SET deleted_at = NOW(),
		     deletion_reason = $1,
		     deleted_by = $2,
		     email = $3
		 WHERE id = $4 AND deleted_at IS NULL`,
		reason, deletedBy, email+"_deleted_"+id.String()[:8], id,
	)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

// scanUser is a helper to scan a single user row.
func (r *userRepository) scanUser(ctx context.Context, query string, args ...any) (*models.User, error) {
	var user models.User
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&user.ID, &user.Email, &user.Password, &user.IsVerified,
		&user.DeletedAt, &user.DeletionReason, &user.DeletedBy,
		&user.CreatedAt, &user.PasswordUpdatedAt, &user.EmailUpdatedAt,
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
	return errors.As(err, &pgErr) && pgErr.Code == "23505" // unique_violation
}
