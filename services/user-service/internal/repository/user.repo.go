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
	LastSeenAt(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]time.Time, error)
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

// profileColumns и profileJoins - общие для чтения профиля со статусом и фигурой; истёкший статус не читается
const (
	profileColumns = `p.user_id, p.username, p.display_name, p.bio, p.avatar_url, p.last_seen_at, p.created_at, p.updated_at,
		s.icon, s.text, s.background, s.foreground, s.expires_at, sh.shape, sh.color`
	profileJoins = `LEFT JOIN user_statuses s ON s.user_id = p.user_id AND (s.expires_at IS NULL OR s.expires_at > NOW())
		LEFT JOIN user_avatar_shapes sh ON sh.user_id = p.user_id`
)

func scanProfile(row pgx.Row) (*models.Profile, error) {
	var (
		p                          models.Profile
		stIcon, stText, stBg, stFg *string
		stExpires                  *time.Time
		shShape, shColor           *string
	)
	if err := row.Scan(
		&p.UserID, &p.Username, &p.DisplayName, &p.Bio, &p.AvatarURL,
		&p.LastSeenAt, &p.CreatedAt, &p.UpdatedAt,
		&stIcon, &stText, &stBg, &stFg, &stExpires, &shShape, &shColor,
	); err != nil {
		return nil, err
	}
	if stIcon != nil && stText != nil && stBg != nil && stFg != nil {
		p.Status = &models.Status{Icon: *stIcon, Text: *stText, Background: *stBg, Foreground: *stFg, ExpiresAt: stExpires}
	}
	if shShape != nil && shColor != nil {
		p.AvatarShape = &models.AvatarShape{Shape: *shShape, Color: *shColor}
	}
	return &p, nil
}

func (r *profileRepository) GetAllBySearch(ctx context.Context, query string, limit, offset int) ([]models.Profile, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+profileColumns+`
		 FROM profiles p `+profileJoins+`
		 WHERE p.username ILIKE $1 AND p.deleted_at IS NULL
		 ORDER BY p.username ASC
		 LIMIT $2 OFFSET $3`,
		query+"%", limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("search profiles: %w", err)
	}
	defer rows.Close()

	profiles := make([]models.Profile, 0)
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, fmt.Errorf("scan profile row: %w", err)
		}
		profiles = append(profiles, *p)
	}
	return profiles, nil
}

func (r *profileRepository) FindByID(ctx context.Context, userID uuid.UUID) (*models.Profile, error) {
	p, err := scanProfile(r.pool.QueryRow(ctx,
		`SELECT `+profileColumns+`
		 FROM profiles p `+profileJoins+`
		 WHERE p.user_id = $1 AND p.deleted_at IS NULL`,
		userID,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrNotFound
		}
		return nil, fmt.Errorf("find profile by id: %w", err)
	}
	return p, nil
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

// UpdateLastSeenAt двигает last_seen_at только вперёд: повтор или запоздавшее событие ничего не меняют
func (r *profileRepository) UpdateLastSeenAt(ctx context.Context, userID uuid.UUID, lastSeen time.Time) error {
	if _, err := r.pool.Exec(ctx,
		`UPDATE profiles SET last_seen_at = $1 WHERE user_id = $2 AND deleted_at IS NULL AND last_seen_at < $1`,
		lastSeen, userID,
	); err != nil {
		return fmt.Errorf("update last_seen_at: %w", err)
	}
	return nil
}

func (r *profileRepository) LastSeenAt(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]time.Time, error) {
	res := make(map[uuid.UUID]time.Time, len(userIDs))
	if len(userIDs) == 0 {
		return res, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT user_id, last_seen_at FROM profiles WHERE user_id = ANY($1) AND deleted_at IS NULL`, userIDs)
	if err != nil {
		return nil, fmt.Errorf("select last_seen_at: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id uuid.UUID
			at time.Time
		)
		if err := rows.Scan(&id, &at); err != nil {
			return nil, fmt.Errorf("scan last_seen_at: %w", err)
		}
		res[id] = at
	}
	return res, rows.Err()
}

func isDuplicateKey(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
