package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
)

type BlockRepository interface {
	GetBlockedUsers(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.BlockedEntry, error)
	CheckBlock(ctx context.Context, userID, targetID uuid.UUID) (bool, error)
	CheckBlockBidirectional(ctx context.Context, userA, userB uuid.UUID) (blockedByA, blockedByB bool, err error)
	BlockUser(ctx context.Context, userID, blockedUserID uuid.UUID) error
	UnblockUser(ctx context.Context, userID, blockedUserID uuid.UUID) error
}

type blockRepository struct {
	pool *pgxpool.Pool
}

func NewBlockRepository(pool *pgxpool.Pool) BlockRepository {
	return &blockRepository{pool: pool}
}

func (r *blockRepository) GetBlockedUsers(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.BlockedEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT b.blocked_user_id, p.username, p.display_name, p.avatar_url, b.created_at
		 FROM user_blocks b
		 JOIN profiles p ON p.user_id = b.blocked_user_id AND p.deleted_at IS NULL
		 WHERE b.user_id = $1
		 ORDER BY b.created_at DESC
		 LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("get blocked users: %w", err)
	}
	defer rows.Close()

	result := make([]models.BlockedEntry, 0)
	for rows.Next() {
		var e models.BlockedEntry
		if err := rows.Scan(
			&e.BlockedUserID, &e.Username, &e.DisplayName, &e.AvatarURL, &e.BlockedAt,
		); err != nil {
			return nil, fmt.Errorf("scan blocked entry: %w", err)
		}
		result = append(result, e)
	}
	return result, nil
}

func (r *blockRepository) CheckBlock(ctx context.Context, userID, targetID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM user_blocks WHERE user_id = $1 AND blocked_user_id = $2)`,
		userID, targetID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check block: %w", err)
	}
	return exists, nil
}

func (r *blockRepository) BlockUser(ctx context.Context, userID, blockedUserID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO user_blocks (user_id, blocked_user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, blockedUserID,
	)
	if err != nil {
		return fmt.Errorf("block user: %w", err)
	}
	return nil
}

func (r *blockRepository) CheckBlockBidirectional(ctx context.Context, userA, userB uuid.UUID) (blockedByA, blockedByB bool, err error) {
	err = r.pool.QueryRow(ctx,
		`SELECT
			EXISTS(SELECT 1 FROM user_blocks WHERE user_id = $1 AND blocked_user_id = $2),
			EXISTS(SELECT 1 FROM user_blocks WHERE user_id = $2 AND blocked_user_id = $1)`,
		userA, userB,
	).Scan(&blockedByA, &blockedByB)
	if err != nil {
		return false, false, fmt.Errorf("check block bidirectional: %w", err)
	}
	return
}

func (r *blockRepository) UnblockUser(ctx context.Context, userID, blockedUserID uuid.UUID) error {
	ct, err := r.pool.Exec(ctx,
		`DELETE FROM user_blocks WHERE user_id = $1 AND blocked_user_id = $2`,
		userID, blockedUserID,
	)
	if err != nil {
		return fmt.Errorf("unblock user: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}
