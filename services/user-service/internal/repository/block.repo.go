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
	GetAllBlocks(ctx context.Context, userID uuid.UUID) ([]models.Block, error)
	CheckBlock(ctx context.Context, userID, targetID uuid.UUID) (bool, error)
	BlockUser(ctx context.Context, block *models.Block) error
	UnblockUser(ctx context.Context, userID, blockedUserID uuid.UUID) error
}

type blockRepository struct {
	pool *pgxpool.Pool
}

func NewBlockRepository(pool *pgxpool.Pool) BlockRepository {
	return &blockRepository{pool: pool}
}

func (r *blockRepository) GetAllBlocks(ctx context.Context, userID uuid.UUID) ([]models.Block, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT b.id, b.profile_id, b.blocked_profile_id, b.created_at,
		        p.id, p.username, p.full_name, p.bio, p.avatar_url, p.birth_date, p.last_seen, p.created_at, p.updated_at
		 FROM blocks b
		 JOIN profiles p ON p.id = b.blocked_profile_id
		 WHERE b.profile_id = $1
		 ORDER BY b.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get all blocks: %w", err)
	}
	defer rows.Close()

	var blocks []models.Block
	for rows.Next() {
		var b models.Block
		var p models.Profile
		var bio, avatarURL *string

		err := rows.Scan(
			&b.ID, &b.ProfileID, &b.BlockedProfileID, &b.CreatedAt,
			&p.ID, &p.Username, &p.FullName, &bio, &avatarURL, &p.BirthDate, &p.LastSeen, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan block row: %w", err)
		}
		if bio != nil {
			p.Bio = *bio
		}
		if avatarURL != nil {
			p.AvatarURL = *avatarURL
		}
		b.BlockedProfile = &p
		blocks = append(blocks, b)
	}

	if blocks == nil {
		blocks = []models.Block{}
	}

	return blocks, nil
}

func (r *blockRepository) CheckBlock(ctx context.Context, userID, targetID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM blocks WHERE profile_id = $1 AND blocked_profile_id = $2)`,
		userID, targetID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check block: %w", err)
	}
	return exists, nil
}

func (r *blockRepository) BlockUser(ctx context.Context, block *models.Block) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO blocks (profile_id, blocked_profile_id)
		 VALUES ($1, $2)
		 RETURNING id, created_at`,
		block.ProfileID, block.BlockedProfileID,
	).Scan(&block.ID, &block.CreatedAt)
	if err != nil {
		if isDuplicateKey(err) {
			return nil
		}
		return fmt.Errorf("block user: %w", err)
	}
	return nil
}

func (r *blockRepository) UnblockUser(ctx context.Context, userID, blockedUserID uuid.UUID) error {
	ct, err := r.pool.Exec(ctx,
		`DELETE FROM blocks WHERE profile_id = $1 AND blocked_profile_id = $2`,
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
