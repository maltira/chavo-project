package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
)

// MaxReactionUserIDs - сколько первых user_id возвращать в сводке по одному эмодзи
const MaxReactionUserIDs = 20

type ReactionRepository interface {
	// Add добавляет реакцию пользователя; false - такая реакция уже стояла
	Add(ctx context.Context, q DBTX, messageID, userID uuid.UUID, emoji string) (bool, error)
	// Remove снимает реакцию пользователя; false - реакции не было
	Remove(ctx context.Context, q DBTX, messageID, userID uuid.UUID, emoji string) (bool, error)
	CountDistinct(ctx context.Context, q DBTX, messageID uuid.UUID) (int, error)
	CountByUser(ctx context.Context, q DBTX, messageID, userID uuid.UUID) (int, error)
	ListByMessages(ctx context.Context, q DBTX, messageIDs []uuid.UUID, viewerID uuid.UUID) (map[uuid.UUID][]models.ReactionSummary, error)
	DeleteByMessage(ctx context.Context, q DBTX, messageID uuid.UUID) error
}

type reactionRepository struct{}

func NewReactionRepository() ReactionRepository {
	return &reactionRepository{}
}

func (r *reactionRepository) Add(ctx context.Context, q DBTX, messageID, userID uuid.UUID, emoji string) (bool, error) {
	ct, err := q.Exec(ctx,
		`INSERT INTO message_reactions (message_id, user_id, emoji)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (message_id, user_id, emoji) DO NOTHING`,
		messageID, userID, emoji)
	if err != nil {
		return false, fmt.Errorf("add reaction: %w", err)
	}
	return ct.RowsAffected() == 1, nil
}

func (r *reactionRepository) Remove(ctx context.Context, q DBTX, messageID, userID uuid.UUID, emoji string) (bool, error) {
	ct, err := q.Exec(ctx,
		`DELETE FROM message_reactions WHERE message_id = $1 AND user_id = $2 AND emoji = $3`,
		messageID, userID, emoji)
	if err != nil {
		return false, fmt.Errorf("remove reaction: %w", err)
	}
	return ct.RowsAffected() > 0, nil
}

func (r *reactionRepository) CountDistinct(ctx context.Context, q DBTX, messageID uuid.UUID) (int, error) {
	var n int
	if err := q.QueryRow(ctx,
		`SELECT COUNT(DISTINCT emoji) FROM message_reactions WHERE message_id = $1`, messageID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count distinct reactions: %w", err)
	}
	return n, nil
}

func (r *reactionRepository) CountByUser(ctx context.Context, q DBTX, messageID, userID uuid.UUID) (int, error) {
	var n int
	if err := q.QueryRow(ctx,
		`SELECT COUNT(*) FROM message_reactions WHERE message_id = $1 AND user_id = $2`, messageID, userID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count user reactions: %w", err)
	}
	return n, nil
}

func (r *reactionRepository) ListByMessages(ctx context.Context, q DBTX, messageIDs []uuid.UUID, viewerID uuid.UUID) (map[uuid.UUID][]models.ReactionSummary, error) {
	res := make(map[uuid.UUID][]models.ReactionSummary)
	if len(messageIDs) == 0 {
		return res, nil
	}
	rows, err := q.Query(ctx,
		`WITH ranked AS (
		    SELECT message_id, emoji, user_id, created_at,
		           ROW_NUMBER() OVER (PARTITION BY message_id, emoji ORDER BY created_at ASC, user_id ASC) AS rn
		    FROM message_reactions
		    WHERE message_id = ANY($1)
		)
		SELECT message_id,
		       emoji,
		       COUNT(*)::int AS cnt,
		       BOOL_OR(user_id = $2) AS me,
		       ARRAY_AGG(user_id ORDER BY created_at ASC, user_id ASC) FILTER (WHERE rn <= $3) AS user_ids
		FROM ranked
		GROUP BY message_id, emoji
		ORDER BY message_id, MIN(created_at) ASC, emoji ASC`,
		messageIDs, viewerID, MaxReactionUserIDs)
	if err != nil {
		return nil, fmt.Errorf("list reactions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			msgID uuid.UUID
			s     models.ReactionSummary
		)
		if err := rows.Scan(&msgID, &s.Emoji, &s.Count, &s.Me, &s.UserIDs); err != nil {
			return nil, fmt.Errorf("scan reaction: %w", err)
		}
		res[msgID] = append(res[msgID], s)
	}
	return res, rows.Err()
}

func (r *reactionRepository) DeleteByMessage(ctx context.Context, q DBTX, messageID uuid.UUID) error {
	if _, err := q.Exec(ctx, `DELETE FROM message_reactions WHERE message_id = $1`, messageID); err != nil {
		return fmt.Errorf("delete message reactions: %w", err)
	}
	return nil
}
