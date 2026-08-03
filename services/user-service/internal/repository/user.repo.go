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
	Update(ctx context.Context, userID uuid.UUID, updates map[string]any) error
	GetAllBySearch(ctx context.Context, query string, limit int) ([]models.Profile, error)
	FindByID(ctx context.Context, userID uuid.UUID) (*models.Profile, error)
	UsernameExists(ctx context.Context, username string) (bool, error)
	UpdateLastSeen(ctx context.Context, userID uuid.UUID, lastSeen time.Time) error
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
		`INSERT INTO profiles (id, username, full_name, avatar_url)
		 VALUES ($1, $2, $3, $4)`,
		profile.ID, profile.Username, profile.FullName, profile.AvatarURL,
	)
	if err != nil {
		if isDuplicateKey(err) {
			return apperror.ErrUsernameExists
		}
		return fmt.Errorf("create profile: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO settings (profile_id) VALUES ($1)`,
		settings.ProfileID,
	)
	if err != nil {
		return fmt.Errorf("create settings: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r *profileRepository) Update(ctx context.Context, userID uuid.UUID, updates map[string]any) error {
	if len(updates) == 0 {
		return apperror.ErrNoColumnsToUpdate
	}

	setClauses := make([]string, 0, len(updates))
	args := make([]any, 0, len(updates)+1)
	argIdx := 1

	for col, val := range updates {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, argIdx))
		args = append(args, val)
		argIdx++
	}

	args = append(args, userID)
	query := fmt.Sprintf("UPDATE profiles SET %s WHERE id = $%d", strings.Join(setClauses, ", "), argIdx)

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

func (r *profileRepository) GetAllBySearch(ctx context.Context, query string, limit int) ([]models.Profile, error) {
	if len(query) < 3 {
		return []models.Profile{}, nil
	}

	cleanQuery := strings.TrimSpace(strings.TrimPrefix(query, "@"))

	rows, err := r.pool.Query(ctx,
		`SELECT p.id, p.username, p.full_name, p.bio, p.avatar_url, p.birth_date, p.last_seen, p.created_at, p.updated_at,
		        s.id, s.profile_id, s.show_online_status, s.show_birth_date
		 FROM profiles p
		 LEFT JOIN settings s ON s.profile_id = p.id
		 WHERE p.username ILIKE $1
		 ORDER BY p.username ASC
		 LIMIT $2`,
		cleanQuery+"%", limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search profiles: %w", err)
	}
	defer rows.Close()

	var profiles []models.Profile
	for rows.Next() {
		var p models.Profile
		var s models.Settings
		var bio, avatarURL *string

		err := rows.Scan(
			&p.ID, &p.Username, &p.FullName, &bio, &avatarURL, &p.BirthDate, &p.LastSeen, &p.CreatedAt, &p.UpdatedAt,
			&s.ID, &s.ProfileID, &s.ShowOnlineStatus, &s.ShowBirthDate,
		)
		if err != nil {
			return nil, fmt.Errorf("scan profile row: %w", err)
		}
		if bio != nil {
			p.Bio = *bio
		}
		if avatarURL != nil {
			p.AvatarURL = *avatarURL
		}
		p.Settings = &s
		profiles = append(profiles, p)
	}

	if profiles == nil {
		profiles = []models.Profile{}
	}

	return profiles, nil
}

func (r *profileRepository) FindByID(ctx context.Context, userID uuid.UUID) (*models.Profile, error) {
	var p models.Profile
	var s models.Settings
	var bio, avatarURL *string

	err := r.pool.QueryRow(ctx,
		`SELECT p.id, p.username, p.full_name, p.bio, p.avatar_url, p.birth_date, p.last_seen, p.created_at, p.updated_at,
		        s.id, s.profile_id, s.show_online_status, s.show_birth_date
		 FROM profiles p
		 LEFT JOIN settings s ON s.profile_id = p.id
		 WHERE p.id = $1`,
		userID,
	).Scan(
		&p.ID, &p.Username, &p.FullName, &bio, &avatarURL, &p.BirthDate, &p.LastSeen, &p.CreatedAt, &p.UpdatedAt,
		&s.ID, &s.ProfileID, &s.ShowOnlineStatus, &s.ShowBirthDate,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrNotFound
		}
		return nil, fmt.Errorf("find profile by id: %w", err)
	}

	if bio != nil {
		p.Bio = *bio
	}
	if avatarURL != nil {
		p.AvatarURL = *avatarURL
	}
	p.Settings = &s

	return &p, nil
}

func (r *profileRepository) UsernameExists(ctx context.Context, username string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM profiles WHERE username = $1)`, username,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check username exists: %w", err)
	}
	return exists, nil
}

func (r *profileRepository) UpdateLastSeen(ctx context.Context, userID uuid.UUID, lastSeen time.Time) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE profiles SET last_seen = $1 WHERE id = $2`, lastSeen, userID,
	)
	if err != nil {
		return fmt.Errorf("update last_seen: %w", err)
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
