package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
)

type SettingsRepository interface {
	GetSettings(ctx context.Context, userID uuid.UUID) (*models.Settings, error)
	UpdateSettings(ctx context.Context, userID uuid.UUID, updates map[string]any) error
}

type settingsRepository struct {
	pool *pgxpool.Pool
}

func NewSettingsRepository(pool *pgxpool.Pool) SettingsRepository {
	return &settingsRepository{pool: pool}
}

func (r *settingsRepository) GetSettings(ctx context.Context, userID uuid.UUID) (*models.Settings, error) {
	var s models.Settings
	err := r.pool.QueryRow(ctx,
		`SELECT user_id, system_language, theme, allow_group_invites, show_online_status, created_at, updated_at
		 FROM user_settings WHERE user_id = $1`, userID,
	).Scan(
		&s.UserID, &s.SystemLanguage, &s.Theme,
		&s.AllowGroupInvites, &s.ShowOnlineStatus,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrNotFound
		}
		return nil, fmt.Errorf("get settings: %w", err)
	}
	return &s, nil
}

var allowedSettingsColumns = map[string]bool{
	"system_language": true, "theme": true,
	"allow_group_invites": true, "show_online_status": true,
}

func (r *settingsRepository) UpdateSettings(ctx context.Context, userID uuid.UUID, updates map[string]any) error {
	for col := range updates {
		if !allowedSettingsColumns[col] {
			return fmt.Errorf("update settings: unknown column %q", col)
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
		"UPDATE user_settings SET %s WHERE user_id = $%d",
		strings.Join(setClauses, ", "), argIdx,
	)

	ct, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update settings: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}
