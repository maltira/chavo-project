package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OutboxRecord - неопубликованное событие auth-service; Payload - готовый JSON-конверт
type OutboxRecord struct {
	ID       int64
	Topic    string
	Key      string
	Payload  []byte
	Attempts int
}

type OutboxRepository interface {
	InsertTx(ctx context.Context, tx pgx.Tx, topic, key string, event any) error
	// TryLockTx берёт транзакционный advisory-lock, чтобы события публиковались строго в порядке id
	TryLockTx(ctx context.Context, tx pgx.Tx) (bool, error)
	FetchUnpublishedTx(ctx context.Context, tx pgx.Tx, limit int) ([]OutboxRecord, error)
	MarkPublishedTx(ctx context.Context, tx pgx.Tx, id int64) error
	RecordFailureTx(ctx context.Context, tx pgx.Tx, id int64) error
	DeletePublishedBefore(ctx context.Context, olderThan time.Duration) (int64, error)
}

const outboxAdvisoryLockKey int64 = 7013842002

type outboxRepository struct {
	pool *pgxpool.Pool
}

func NewOutboxRepository(pool *pgxpool.Pool) OutboxRepository {
	return &outboxRepository{pool: pool}
}

func (r *outboxRepository) InsertTx(ctx context.Context, tx pgx.Tx, topic, key string, event any) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal outbox event: %w", err)
	}
	if _, err = tx.Exec(ctx,
		`INSERT INTO outbox_events (topic, event_key, payload) VALUES ($1, $2, $3)`,
		topic, key, payload,
	); err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}

func (r *outboxRepository) TryLockTx(ctx context.Context, tx pgx.Tx) (bool, error) {
	var ok bool
	if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, outboxAdvisoryLockKey).Scan(&ok); err != nil {
		return false, fmt.Errorf("outbox advisory lock: %w", err)
	}
	return ok, nil
}

func (r *outboxRepository) FetchUnpublishedTx(ctx context.Context, tx pgx.Tx, limit int) ([]OutboxRecord, error) {
	rows, err := tx.Query(ctx,
		`SELECT id, topic, event_key, payload, attempts FROM outbox_events
		 WHERE published_at IS NULL AND failed_at IS NULL ORDER BY id LIMIT $1`, limit)
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

func (r *outboxRepository) MarkPublishedTx(ctx context.Context, tx pgx.Tx, id int64) error {
	if _, err := tx.Exec(ctx, `UPDATE outbox_events SET published_at = clock_timestamp() WHERE id = $1`, id); err != nil {
		return fmt.Errorf("mark outbox published: %w", err)
	}
	return nil
}

func (r *outboxRepository) RecordFailureTx(ctx context.Context, tx pgx.Tx, id int64) error {
	if _, err := tx.Exec(ctx, `UPDATE outbox_events SET attempts = attempts + 1 WHERE id = $1`, id); err != nil {
		return fmt.Errorf("record outbox failure: %w", err)
	}
	return nil
}

func (r *outboxRepository) DeletePublishedBefore(ctx context.Context, olderThan time.Duration) (int64, error) {
	ct, err := r.pool.Exec(ctx,
		`DELETE FROM outbox_events
		 WHERE published_at IS NOT NULL AND published_at < clock_timestamp() - ($1::bigint * interval '1 second')`,
		int64(olderThan.Seconds()),
	)
	if err != nil {
		return 0, fmt.Errorf("purge outbox: %w", err)
	}
	return ct.RowsAffected(), nil
}
