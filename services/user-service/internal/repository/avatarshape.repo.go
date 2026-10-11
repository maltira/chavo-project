package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
)

// AvatarShapeRepository - фигура позади аватара: только установить (заменить целиком) или удалить
type AvatarShapeRepository interface {
	Set(ctx context.Context, userID uuid.UUID, sh models.AvatarShape) error
	Clear(ctx context.Context, userID uuid.UUID) error
}

type avatarShapeRepository struct {
	pool *pgxpool.Pool
}

func NewAvatarShapeRepository(pool *pgxpool.Pool) AvatarShapeRepository {
	return &avatarShapeRepository{pool: pool}
}

func (r *avatarShapeRepository) Set(ctx context.Context, userID uuid.UUID, sh models.AvatarShape) error {
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO user_avatar_shapes (user_id, shape, color, set_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (user_id) DO UPDATE SET shape = EXCLUDED.shape, color = EXCLUDED.color, set_at = NOW()`,
		userID, sh.Shape, sh.Color,
	); err != nil {
		return fmt.Errorf("set avatar shape: %w", err)
	}
	return nil
}

func (r *avatarShapeRepository) Clear(ctx context.Context, userID uuid.UUID) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM user_avatar_shapes WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("clear avatar shape: %w", err)
	}
	return nil
}
