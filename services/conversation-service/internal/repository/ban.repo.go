package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
)

type BanRepository interface {
	Ban(ctx context.Context, q DBTX, convID, userID, bannedBy uuid.UUID) error
	// Unban возвращает false, если пользователь не был забанен.
	Unban(ctx context.Context, q DBTX, convID, userID uuid.UUID) (bool, error)
	IsBanned(ctx context.Context, q DBTX, convID, userID uuid.UUID) (bool, error)
	List(ctx context.Context, q DBTX, convID uuid.UUID, limit, offset int) ([]models.Ban, error)
}

type banRepository struct{}

func NewBanRepository() BanRepository {
	return &banRepository{}
}

func (r *banRepository) Ban(ctx context.Context, q DBTX, convID, userID, bannedBy uuid.UUID) error {
	if _, err := q.Exec(ctx,
		`INSERT INTO conversation_bans (conversation_id, user_id, banned_by) VALUES ($1, $2, $3)
		 ON CONFLICT (conversation_id, user_id) DO NOTHING`,
		convID, userID, bannedBy,
	); err != nil {
		return fmt.Errorf("ban user: %w", err)
	}
	return nil
}

func (r *banRepository) Unban(ctx context.Context, q DBTX, convID, userID uuid.UUID) (bool, error) {
	ct, err := q.Exec(ctx, `DELETE FROM conversation_bans WHERE conversation_id = $1 AND user_id = $2`, convID, userID)
	if err != nil {
		return false, fmt.Errorf("unban user: %w", err)
	}
	return ct.RowsAffected() == 1, nil
}

func (r *banRepository) IsBanned(ctx context.Context, q DBTX, convID, userID uuid.UUID) (bool, error) {
	var banned bool
	if err := q.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM conversation_bans WHERE conversation_id = $1 AND user_id = $2)`,
		convID, userID,
	).Scan(&banned); err != nil {
		return false, fmt.Errorf("check ban: %w", err)
	}
	return banned, nil
}

func (r *banRepository) List(ctx context.Context, q DBTX, convID uuid.UUID, limit, offset int) ([]models.Ban, error) {
	rows, err := q.Query(ctx,
		`SELECT user_id, banned_by, created_at FROM conversation_bans
		 WHERE conversation_id = $1 ORDER BY created_at, user_id LIMIT $2 OFFSET $3`,
		convID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list bans: %w", err)
	}
	defer rows.Close()

	res := make([]models.Ban, 0)
	for rows.Next() {
		var b models.Ban
		if err := rows.Scan(&b.UserID, &b.BannedBy, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan ban: %w", err)
		}
		res = append(res, b)
	}
	return res, rows.Err()
}
