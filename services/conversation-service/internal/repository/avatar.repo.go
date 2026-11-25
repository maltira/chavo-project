package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
)

// AvatarPhoto - фото из истории аватарок группы; текущее - самое новое
type AvatarPhoto struct {
	ID        uuid.UUID
	URL       string
	CreatedAt time.Time
	IsCurrent bool
}

// AvatarHistoryRepository - история аватарок группы. Текущая - самая новая строка; её адрес дублируется
// в conversations.avatar_url. Изменения выполняются в транзакции под блокировкой строки группы (Lock).
type AvatarHistoryRepository interface {
	// List - фото от новых к старым
	List(ctx context.Context, q DBTX, convID uuid.UUID) ([]AvatarPhoto, error)
	// Add делает фото текущим; возвращает адреса выпавших из истории (старше limit последних)
	Add(ctx context.Context, q DBTX, convID uuid.UUID, url string, limit int) ([]string, error)
	// Select поднимает фото из истории наверх и делает текущим; ErrNotFound, если такого нет
	Select(ctx context.Context, q DBTX, convID, photoID uuid.UUID) error
	// Delete убирает фото (nil - текущее) и возвращает его адрес; текущим становится предыдущее или никакого;
	// ErrNotFound, если удалять нечего
	Delete(ctx context.Context, q DBTX, convID uuid.UUID, photoID *uuid.UUID) (string, error)
}

type avatarHistoryRepository struct{}

func NewAvatarHistoryRepository() AvatarHistoryRepository {
	return &avatarHistoryRepository{}
}

// syncCurrent записывает в группу адрес самой новой строки истории (NULL, если истории нет)
func syncCurrent(ctx context.Context, q DBTX, convID uuid.UUID) error {
	if _, err := q.Exec(ctx,
		`UPDATE conversations SET updated_at = clock_timestamp(),
		        avatar_url = (SELECT avatar_url FROM conversation_avatars WHERE conversation_id = $1
		                      ORDER BY created_at DESC, id DESC LIMIT 1)
		 WHERE id = $1`, convID); err != nil {
		return fmt.Errorf("sync current group avatar: %w", err)
	}
	return nil
}

func (r *avatarHistoryRepository) List(ctx context.Context, q DBTX, convID uuid.UUID) ([]AvatarPhoto, error) {
	rows, err := q.Query(ctx,
		`SELECT id, avatar_url, created_at FROM conversation_avatars WHERE conversation_id = $1
		 ORDER BY created_at DESC, id DESC`, convID)
	if err != nil {
		return nil, fmt.Errorf("list group avatars: %w", err)
	}
	defer rows.Close()

	res := make([]AvatarPhoto, 0)
	for rows.Next() {
		var p AvatarPhoto
		if err := rows.Scan(&p.ID, &p.URL, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan group avatar: %w", err)
		}
		p.IsCurrent = len(res) == 0
		res = append(res, p)
	}
	return res, rows.Err()
}

func (r *avatarHistoryRepository) Add(ctx context.Context, q DBTX, convID uuid.UUID, url string, limit int) ([]string, error) {
	if _, err := q.Exec(ctx, `INSERT INTO conversation_avatars (conversation_id, avatar_url) VALUES ($1, $2)`, convID, url); err != nil {
		return nil, fmt.Errorf("insert group avatar: %w", err)
	}
	rows, err := q.Query(ctx,
		`DELETE FROM conversation_avatars WHERE id IN (
		     SELECT id FROM conversation_avatars WHERE conversation_id = $1
		     ORDER BY created_at DESC, id DESC OFFSET $2)
		 RETURNING avatar_url`, convID, limit)
	if err != nil {
		return nil, fmt.Errorf("trim group avatars: %w", err)
	}
	var evicted []string
	for rows.Next() {
		var u string
		if err = rows.Scan(&u); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan evicted group avatar: %w", err)
		}
		evicted = append(evicted, u)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return evicted, syncCurrent(ctx, q, convID)
}

func (r *avatarHistoryRepository) Select(ctx context.Context, q DBTX, convID, photoID uuid.UUID) error {
	ct, err := q.Exec(ctx,
		`UPDATE conversation_avatars SET created_at = clock_timestamp() WHERE id = $1 AND conversation_id = $2`, photoID, convID)
	if err != nil {
		return fmt.Errorf("select group avatar: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return syncCurrent(ctx, q, convID)
}

func (r *avatarHistoryRepository) Delete(ctx context.Context, q DBTX, convID uuid.UUID, photoID *uuid.UUID) (string, error) {
	var removed string
	err := q.QueryRow(ctx,
		`DELETE FROM conversation_avatars WHERE id = COALESCE($2::uuid,
		     (SELECT id FROM conversation_avatars WHERE conversation_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1))
		   AND conversation_id = $1
		 RETURNING avatar_url`, convID, photoID).Scan(&removed)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperror.ErrNotFound
		}
		return "", fmt.Errorf("delete group avatar: %w", err)
	}
	return removed, syncCurrent(ctx, q, convID)
}
