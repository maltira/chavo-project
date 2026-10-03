package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// OutboxRecord — неопубликованное событие; Payload — готовый JSON-конверт.
type OutboxRecord struct {
	ID       int64
	Topic    string
	Key      string
	Payload  []byte
	Attempts int
}

type OutboxRepository interface {
	Insert(ctx context.Context, q DBTX, topic, key string, event any) error

	// ScrubMessageContent удаляет шифртекст сообщения из всех его событий (при удалении сообщения).
	ScrubMessageContent(ctx context.Context, q DBTX, messageID uuid.UUID) error

	// TryLock берёт транзакционный advisory-lock: одновременно публикует только один воркер,
	// поэтому события уходят строго в порядке id.
	TryLock(ctx context.Context, q DBTX) (bool, error)
	FetchUnpublished(ctx context.Context, q DBTX, limit int) ([]OutboxRecord, error)
	MarkPublished(ctx context.Context, q DBTX, id int64) error
	RecordFailure(ctx context.Context, q DBTX, id int64) error
	DeletePublishedBefore(ctx context.Context, q DBTX, olderThan time.Duration) (int64, error)
}

const outboxAdvisoryLockKey int64 = 7013842001

type outboxRepository struct{}

func NewOutboxRepository() OutboxRepository {
	return &outboxRepository{}
}

func (r *outboxRepository) Insert(ctx context.Context, q DBTX, topic, key string, event any) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal outbox event: %w", err)
	}
	if _, err = q.Exec(ctx,
		`INSERT INTO outbox_events (topic, event_key, payload) VALUES ($1, $2, $3)`,
		topic, key, payload,
	); err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}

func (r *outboxRepository) ScrubMessageContent(ctx context.Context, q DBTX, messageID uuid.UUID) error {
	if _, err := q.Exec(ctx,
		`UPDATE outbox_events SET payload = payload #- '{payload,content_enc}'
		 WHERE topic = 'message-events' AND payload->'payload'->>'message_id' = $1`,
		messageID.String(),
	); err != nil {
		return fmt.Errorf("scrub outbox message content: %w", err)
	}
	return nil
}

func (r *outboxRepository) TryLock(ctx context.Context, q DBTX) (bool, error) {
	var ok bool
	if err := q.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, outboxAdvisoryLockKey).Scan(&ok); err != nil {
		return false, fmt.Errorf("outbox advisory lock: %w", err)
	}
	return ok, nil
}

func (r *outboxRepository) FetchUnpublished(ctx context.Context, q DBTX, limit int) ([]OutboxRecord, error) {
	rows, err := q.Query(ctx,
		`SELECT id, topic, event_key, payload, attempts FROM outbox_events
		 WHERE published_at IS NULL ORDER BY id LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch outbox events: %w", err)
	}
	defer rows.Close()

	res := make([]OutboxRecord, 0, limit)
	for rows.Next() {
		var rec OutboxRecord
		if err := rows.Scan(&rec.ID, &rec.Topic, &rec.Key, &rec.Payload, &rec.Attempts); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		res = append(res, rec)
	}
	return res, rows.Err()
}

func (r *outboxRepository) MarkPublished(ctx context.Context, q DBTX, id int64) error {
	if _, err := q.Exec(ctx, `UPDATE outbox_events SET published_at = clock_timestamp() WHERE id = $1`, id); err != nil {
		return fmt.Errorf("mark outbox published: %w", err)
	}
	return nil
}

func (r *outboxRepository) RecordFailure(ctx context.Context, q DBTX, id int64) error {
	if _, err := q.Exec(ctx, `UPDATE outbox_events SET attempts = attempts + 1 WHERE id = $1`, id); err != nil {
		return fmt.Errorf("record outbox failure: %w", err)
	}
	return nil
}

func (r *outboxRepository) DeletePublishedBefore(ctx context.Context, q DBTX, olderThan time.Duration) (int64, error) {
	ct, err := q.Exec(ctx,
		`DELETE FROM outbox_events
		 WHERE published_at IS NOT NULL AND published_at < clock_timestamp() - ($1::bigint * interval '1 second')`,
		int64(olderThan.Seconds()))
	if err != nil {
		return 0, fmt.Errorf("purge outbox: %w", err)
	}
	return ct.RowsAffected(), nil
}
