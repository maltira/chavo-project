package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
)

// ProfileCoverRepository - обложка профиля: только установить (заменить целиком) или удалить
type ProfileCoverRepository interface {
	Set(ctx context.Context, userID uuid.UUID, cover models.ProfileCover) error
	Clear(ctx context.Context, userID uuid.UUID) error
}

type profileCoverRepository struct {
	pool *pgxpool.Pool
}

func NewProfileCoverRepository(pool *pgxpool.Pool) ProfileCoverRepository {
	return &profileCoverRepository{pool: pool}
}

func (r *profileCoverRepository) Set(ctx context.Context, userID uuid.UUID, cover models.ProfileCover) error {
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO user_profile_covers (user_id, color, pattern, set_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (user_id) DO UPDATE SET color = EXCLUDED.color, pattern = EXCLUDED.pattern, set_at = NOW()`,
		userID, cover.Color, cover.Pattern,
	); err != nil {
		return fmt.Errorf("set profile cover: %w", err)
	}
	return nil
}

func (r *profileCoverRepository) Clear(ctx context.Context, userID uuid.UUID) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM user_profile_covers WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("clear profile cover: %w", err)
	}
	return nil
}
