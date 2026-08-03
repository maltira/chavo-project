package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
)

type SettingsRepository interface {
	GetSettings(ctx context.Context, profileID uuid.UUID) (*models.Settings, error)
	UpdateVisibleStatus(ctx context.Context, userID uuid.UUID, isVisible bool) error
	UpdateVisibleBirthDate(ctx context.Context, userID uuid.UUID, isVisible bool) error
}

type settingsRepository struct {
	pool *pgxpool.Pool
}

func NewSettingsRepository(pool *pgxpool.Pool) SettingsRepository {
	return &settingsRepository{pool: pool}
}

func (r *settingsRepository) GetSettings(ctx context.Context, profileID uuid.UUID) (*models.Settings, error) {
	var s models.Settings
	err := r.pool.QueryRow(ctx,
		`SELECT id, profile_id, show_online_status, show_birth_date
		 FROM settings WHERE profile_id = $1`, profileID,
	).Scan(&s.ID, &s.ProfileID, &s.ShowOnlineStatus, &s.ShowBirthDate)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrNotFound
		}
		return nil, fmt.Errorf("get settings: %w", err)
	}

	return &s, nil
}

func (r *settingsRepository) UpdateVisibleStatus(ctx context.Context, userID uuid.UUID, isVisible bool) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE settings SET show_online_status = $1 WHERE profile_id = $2`,
		isVisible, userID,
	)
	if err != nil {
		return fmt.Errorf("update visible status: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

func (r *settingsRepository) UpdateVisibleBirthDate(ctx context.Context, userID uuid.UUID, isVisible bool) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE settings SET show_birth_date = $1 WHERE profile_id = $2`,
		isVisible, userID,
	)
	if err != nil {
		return fmt.Errorf("update visible birth date: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}
