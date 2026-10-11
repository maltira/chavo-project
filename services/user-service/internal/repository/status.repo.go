package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
)

// StatusRepository — статус пользователя: только установить (заменить целиком) или удалить.
type StatusRepository interface {
	Set(ctx context.Context, userID uuid.UUID, st models.Status) error
	// Clear удаляет статус; false — статуса не было.
	Clear(ctx context.Context, userID uuid.UUID) (bool, error)
}

type statusRepository struct {
	pool *pgxpool.Pool
}

func NewStatusRepository(pool *pgxpool.Pool) StatusRepository {
	return &statusRepository{pool: pool}
}

func (r *statusRepository) Set(ctx context.Context, userID uuid.UUID, st models.Status) error {
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO user_statuses (user_id, icon, text, background, foreground, expires_at, set_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NOW())
		 ON CONFLICT (user_id) DO UPDATE
		   SET icon = EXCLUDED.icon, text = EXCLUDED.text, background = EXCLUDED.background,
		       foreground = EXCLUDED.foreground, expires_at = EXCLUDED.expires_at, set_at = NOW()`,
		userID, st.Icon, st.Text, st.Background, st.Foreground, st.ExpiresAt,
	); err != nil {
		return fmt.Errorf("set status: %w", err)
	}
	return nil
}

func (r *statusRepository) Clear(ctx context.Context, userID uuid.UUID) (bool, error) {
	ct, err := r.pool.Exec(ctx, `DELETE FROM user_statuses WHERE user_id = $1`, userID)
	if err != nil {
		return false, fmt.Errorf("clear status: %w", err)
	}
	return ct.RowsAffected() > 0, nil
}
