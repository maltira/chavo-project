package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
)

type ProfileRepository interface {
	Create(ctx context.Context, profile *models.Profile, settings *models.Settings) error
	Update(ctx context.Context, userID uuid.UUID, updates map[string]string) error
	GetAllBySearch(ctx context.Context, query string, limit, offset int) ([]models.Profile, error)
	FindByID(ctx context.Context, userID uuid.UUID) (*models.Profile, error)
	UsernameExists(ctx context.Context, username string) (bool, error)
	UpdateLastSeenAt(ctx context.Context, userID uuid.UUID, lastSeen time.Time) error
}

type profileRepository struct {
	pool *pgxpool.Pool
}

func NewProfileRepository(pool *pgxpool.Pool) ProfileRepository {
	return &profileRepository{pool: pool}
}

func (r *profileRepository) Create(ctx context.Context, profile *models.Profile, settings *models.Settings) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`INSERT INTO profiles (user_id, username, display_name, bio, avatar_url)
		 VALUES ($1, $2, $3, $4, $5)`,
		profile.UserID, profile.Username, profile.DisplayName, profile.Bio, profile.AvatarURL,
	)
	if err != nil {
		if isDuplicateKey(err) {
			var pgErr *pgconn.PgError
			errors.As(err, &pgErr)
			if pgErr.ConstraintName == "profiles_pkey" {
				return apperror.ErrProfileAlreadyExists
			}
			return apperror.ErrUsernameExists
		}
		return fmt.Errorf("create profile: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO user_settings (user_id) VALUES ($1)`,
		settings.UserID,
	)
	if err != nil {
		return fmt.Errorf("create settings: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

var allowedProfileColumns = map[string]bool{
	"username": true, "display_name": true, "bio": true, "avatar_url": true,
}

func (r *profileRepository) Update(ctx context.Context, userID uuid.UUID, updates map[string]string) error {
	for col := range updates {
		if !allowedProfileColumns[col] {
			return fmt.Errorf("update profile: unknown column %q", col)
		}
	}

	setClauses := make([]string, 0, len(updates)+1)
	args := make([]any, 0, len(updates)+1)
	argIdx := 1

	for col, val := range updates {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, argIdx))
		args = append(args, val)
		argIdx++
	}
	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argIdx))
	args = append(args, time.Now().UTC())
	argIdx++

	args = append(args, userID)
	query := fmt.Sprintf(
		"UPDATE profiles SET %s WHERE user_id = $%d AND deleted_at IS NULL",
		strings.Join(setClauses, ", "), argIdx,
	)

	ct, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		if isDuplicateKey(err) {
			return apperror.ErrUsernameExists
		}
		return fmt.Errorf("update profile: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

func (r *profileRepository) GetAllBySearch(ctx context.Context, query string, limit, offset int) ([]models.Profile, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT user_id, username, display_name, bio, avatar_url, last_seen_at, created_at, updated_at
		 FROM profiles
		 WHERE username ILIKE $1 AND deleted_at IS NULL
		 ORDER BY username ASC
		 LIMIT $2 OFFSET $3`,
		query+"%", limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("search profiles: %w", err)
	}
	defer rows.Close()

	profiles := make([]models.Profile, 0)
	for rows.Next() {
		var p models.Profile
		if err := rows.Scan(
			&p.UserID, &p.Username, &p.DisplayName, &p.Bio, &p.AvatarURL,
			&p.LastSeenAt, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan profile row: %w", err)
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}

func (r *profileRepository) FindByID(ctx context.Context, userID uuid.UUID) (*models.Profile, error) {
	var p models.Profile
	err := r.pool.QueryRow(ctx,
		`SELECT user_id, username, display_name, bio, avatar_url, last_seen_at, created_at, updated_at
		 FROM profiles
		 WHERE user_id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(
		&p.UserID, &p.Username, &p.DisplayName, &p.Bio, &p.AvatarURL,
		&p.LastSeenAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrNotFound
		}
		return nil, fmt.Errorf("find profile by id: %w", err)
	}
	return &p, nil
}

func (r *profileRepository) UsernameExists(ctx context.Context, username string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM profiles WHERE username = $1 AND deleted_at IS NULL)`, username,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check username exists: %w", err)
	}
	return exists, nil
}

func (r *profileRepository) UpdateLastSeenAt(ctx context.Context, userID uuid.UUID, lastSeen time.Time) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE profiles SET last_seen_at = $1, updated_at = $1 WHERE user_id = $2 AND deleted_at IS NULL`,
		lastSeen, userID,
	)
	if err != nil {
		return fmt.Errorf("update last_seen_at: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

func isDuplicateKey(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
