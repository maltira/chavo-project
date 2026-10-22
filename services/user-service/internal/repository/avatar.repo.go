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

// AvatarHistoryRepository - история аватарок профиля. Текущая - самая новая строка; её адрес дублируется
// в profiles.avatar_url. Каждая операция - одна транзакция под блокировкой профиля.
type AvatarHistoryRepository interface {
	// List - фото от новых к старым
	List(ctx context.Context, userID uuid.UUID) ([]models.AvatarPhoto, error)
	// Add делает фото текущим; возвращает адреса выпавших из истории (старше limit последних)
	Add(ctx context.Context, userID uuid.UUID, url string, limit int) ([]string, error)
	// Select поднимает фото из истории наверх и делает текущим; ErrNotFound, если такого нет
	Select(ctx context.Context, userID, photoID uuid.UUID) error
	// Delete убирает фото (nil - текущее) и возвращает его адрес; текущим становится предыдущее или никакого;
	// ErrNotFound, если удалять нечего
	Delete(ctx context.Context, userID uuid.UUID, photoID *uuid.UUID) (string, error)
}

type avatarHistoryRepository struct {
	pool *pgxpool.Pool
}

func NewAvatarHistoryRepository(pool *pgxpool.Pool) AvatarHistoryRepository {
	return &avatarHistoryRepository{pool: pool}
}

// withLockedProfile выполняет fn в транзакции, заблокировав профиль: параллельные изменения истории одного
// пользователя идут по очереди
func (r *avatarHistoryRepository) withLockedProfile(ctx context.Context, userID uuid.UUID, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin avatar tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT user_id FROM profiles WHERE user_id = $1 AND deleted_at IS NULL FOR UPDATE`, userID).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperror.ErrNotFound
		}
		return fmt.Errorf("lock profile: %w", err)
	}
	if err = fn(tx); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit avatar tx: %w", err)
	}
	return nil
}

// syncCurrent записывает в профиль адрес самой новой строки истории (NULL, если истории нет)
func syncCurrent(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	if _, err := tx.Exec(ctx,
		`UPDATE profiles SET updated_at = NOW(),
		        avatar_url = (SELECT avatar_url FROM profile_avatars WHERE user_id = $1
		                      ORDER BY created_at DESC, id DESC LIMIT 1)
		 WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("sync current avatar: %w", err)
	}
	return nil
}

func (r *avatarHistoryRepository) List(ctx context.Context, userID uuid.UUID) ([]models.AvatarPhoto, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, avatar_url, created_at FROM profile_avatars WHERE user_id = $1
		 ORDER BY created_at DESC, id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list avatars: %w", err)
	}
	defer rows.Close()

	res := make([]models.AvatarPhoto, 0)
	for rows.Next() {
		var p models.AvatarPhoto
		if err := rows.Scan(&p.ID, &p.URL, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan avatar: %w", err)
		}
		p.IsCurrent = len(res) == 0
		res = append(res, p)
	}
	return res, rows.Err()
}

func (r *avatarHistoryRepository) Add(ctx context.Context, userID uuid.UUID, url string, limit int) ([]string, error) {
	var evicted []string
	err := r.withLockedProfile(ctx, userID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO profile_avatars (user_id, avatar_url) VALUES ($1, $2)`, userID, url); err != nil {
			return fmt.Errorf("insert avatar: %w", err)
		}
		rows, err := tx.Query(ctx,
			`DELETE FROM profile_avatars WHERE id IN (
			     SELECT id FROM profile_avatars WHERE user_id = $1
			     ORDER BY created_at DESC, id DESC OFFSET $2)
			 RETURNING avatar_url`, userID, limit)
		if err != nil {
			return fmt.Errorf("trim avatars: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var u string
			if err := rows.Scan(&u); err != nil {
				return fmt.Errorf("scan evicted avatar: %w", err)
			}
			evicted = append(evicted, u)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		rows.Close()
		return syncCurrent(ctx, tx, userID)
	})
	if err != nil {
		return nil, err
	}
	return evicted, nil
}

func (r *avatarHistoryRepository) Select(ctx context.Context, userID, photoID uuid.UUID) error {
	return r.withLockedProfile(ctx, userID, func(tx pgx.Tx) error {
		ct, err := tx.Exec(ctx,
			`UPDATE profile_avatars SET created_at = clock_timestamp() WHERE id = $1 AND user_id = $2`, photoID, userID)
		if err != nil {
			return fmt.Errorf("select avatar: %w", err)
		}
		if ct.RowsAffected() == 0 {
			return apperror.ErrNotFound
		}
		return syncCurrent(ctx, tx, userID)
	})
}

func (r *avatarHistoryRepository) Delete(ctx context.Context, userID uuid.UUID, photoID *uuid.UUID) (string, error) {
	var removed string
	err := r.withLockedProfile(ctx, userID, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx,
			`DELETE FROM profile_avatars WHERE id = COALESCE($2::uuid,
			     (SELECT id FROM profile_avatars WHERE user_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1))
			   AND user_id = $1
			 RETURNING avatar_url`, userID, photoID).Scan(&removed)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperror.ErrNotFound
			}
			return fmt.Errorf("delete avatar: %w", err)
		}
		return syncCurrent(ctx, tx, userID)
	})
	if err != nil {
		return "", err
	}
	return removed, nil
}
