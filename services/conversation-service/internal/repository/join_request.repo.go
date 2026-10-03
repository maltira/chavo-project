package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
)

type JoinRequestRepository interface {
	Create(ctx context.Context, q DBTX, convID, userID uuid.UUID) (*models.JoinRequest, error)
	// GetForUpdate блокирует заявку; ErrNotFound, если её нет.
	GetForUpdate(ctx context.Context, q DBTX, requestID uuid.UUID) (*models.JoinRequest, error)
	SetStatus(ctx context.Context, q DBTX, requestID uuid.UUID, status string) error
	List(ctx context.Context, q DBTX, convID uuid.UUID, status string, limit, offset int) ([]models.JoinRequest, error)
	// PendingStatus возвращает статус pending-заявки пользователя или nil.
	PendingStatus(ctx context.Context, q DBTX, convID, userID uuid.UUID) (*string, error)
}

type joinRequestRepository struct{}

func NewJoinRequestRepository() JoinRequestRepository {
	return &joinRequestRepository{}
}

const joinRequestColumns = `id, conversation_id, user_id, request_status, created_at, updated_at`

func scanJoinRequest(row pgx.Row) (*models.JoinRequest, error) {
	var j models.JoinRequest
	if err := row.Scan(&j.ID, &j.ConversationID, &j.UserID, &j.Status, &j.CreatedAt, &j.UpdatedAt); err != nil {
		return nil, err
	}
	return &j, nil
}

// Create возвращает ErrJoinRequestExists, если у пользователя уже есть pending-заявка (partial unique index).
func (r *joinRequestRepository) Create(ctx context.Context, q DBTX, convID, userID uuid.UUID) (*models.JoinRequest, error) {
	j, err := scanJoinRequest(q.QueryRow(ctx,
		`INSERT INTO conversation_join_requests (conversation_id, user_id) VALUES ($1, $2)
		 RETURNING `+joinRequestColumns, convID, userID))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, apperror.ErrJoinRequestExists
		}
		return nil, fmt.Errorf("create join request: %w", err)
	}
	return j, nil
}

func (r *joinRequestRepository) GetForUpdate(ctx context.Context, q DBTX, requestID uuid.UUID) (*models.JoinRequest, error) {
	j, err := scanJoinRequest(q.QueryRow(ctx,
		`SELECT `+joinRequestColumns+` FROM conversation_join_requests WHERE id = $1 FOR UPDATE`, requestID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrNotFound
		}
		return nil, fmt.Errorf("get join request: %w", err)
	}
	return j, nil
}

func (r *joinRequestRepository) SetStatus(ctx context.Context, q DBTX, requestID uuid.UUID, status string) error {
	if _, err := q.Exec(ctx,
		`UPDATE conversation_join_requests SET request_status = $2, updated_at = clock_timestamp() WHERE id = $1`,
		requestID, status); err != nil {
		return fmt.Errorf("set join request status: %w", err)
	}
	return nil
}

func (r *joinRequestRepository) List(ctx context.Context, q DBTX, convID uuid.UUID, status string, limit, offset int) ([]models.JoinRequest, error) {
	rows, err := q.Query(ctx,
		`SELECT `+joinRequestColumns+` FROM conversation_join_requests
		 WHERE conversation_id = $1 AND request_status = $2
		 ORDER BY created_at, id LIMIT $3 OFFSET $4`, convID, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list join requests: %w", err)
	}
	defer rows.Close()

	res := make([]models.JoinRequest, 0)
	for rows.Next() {
		j, err := scanJoinRequest(rows)
		if err != nil {
			return nil, fmt.Errorf("scan join request: %w", err)
		}
		res = append(res, *j)
	}
	return res, rows.Err()
}

func (r *joinRequestRepository) PendingStatus(ctx context.Context, q DBTX, convID, userID uuid.UUID) (*string, error) {
	var status string
	err := q.QueryRow(ctx,
		`SELECT request_status FROM conversation_join_requests
		 WHERE conversation_id = $1 AND user_id = $2 AND request_status = 'pending'`, convID, userID).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("pending join request: %w", err)
	}
	return &status, nil
}
