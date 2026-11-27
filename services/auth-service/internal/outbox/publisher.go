package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/auth-service/internal/repository"
)

// EventPublisher - отправка события в брокер; реализует *kafka.Producer
type EventPublisher interface {
	Publish(ctx context.Context, topic, key string, event any) error
}

// TxRunner абстрагирует запуск транзакции (реализует *pgxpool.Pool и мок в тестах)
type TxRunner interface {
	WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

type poolRunner struct{ pool *pgxpool.Pool }

func (r poolRunner) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin outbox tx: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type Options struct {
	PollInterval   time.Duration
	BatchSize      int
	PublishTimeout time.Duration
	MaxBackoff     time.Duration
	PurgeEvery     time.Duration
	Retention      time.Duration
}

func DefaultOptions() Options {
	return Options{
		PollInterval:   500 * time.Millisecond,
		BatchSize:      100,
		PublishTimeout: 5 * time.Second,
		MaxBackoff:     30 * time.Second,
		PurgeEvery:     time.Hour,
		Retention:      24 * time.Hour,
	}
}

// Publisher переносит события из outbox_events в Kafka (at-least-once, порядок по id)
type Publisher struct {
	tx     TxRunner
	repo   repository.OutboxRepository
	pub    EventPublisher
	log    *zap.Logger
	opts   Options
	wake   chan struct{}
	nextGC time.Time
}

func NewPublisher(pool *pgxpool.Pool, repo repository.OutboxRepository, pub EventPublisher, log *zap.Logger, opts Options) *Publisher {
	return NewPublisherWithRunner(poolRunner{pool: pool}, repo, pub, log, opts)
}

func NewPublisherWithRunner(tx TxRunner, repo repository.OutboxRepository, pub EventPublisher, log *zap.Logger, opts Options) *Publisher {
	return &Publisher{
		tx:   tx,
		repo: repo,
		pub:  pub,
		log:  log,
		opts: opts,
		wake: make(chan struct{}, 1),
	}
}

// Notify будит воркер сразу после коммита новой записи в outbox, не дожидаясь PollInterval
func (p *Publisher) Notify() {
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

// Run крутит публикацию до отмены ctx
func (p *Publisher) Run(ctx context.Context) {
	backoff := time.Duration(0)
	for {
		n, err := p.RunOnce(ctx)

		delay := p.opts.PollInterval
		switch {
		case err != nil:
			backoff = nextBackoff(backoff, p.opts.MaxBackoff)
			delay = backoff
			p.log.Error("Outbox publish failed", zap.Error(err), zap.Duration("retry_in", backoff))
		case n >= p.opts.BatchSize:
			backoff, delay = 0, 0
		default:
			backoff = 0
		}

		p.purgeIfDue(ctx)

		if delay == 0 {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-p.wake:
			timer.Stop()
		case <-timer.C:
		}
	}
}

// RunOnce публикует одну пачку и возвращает число отправленных событий
func (p *Publisher) RunOnce(ctx context.Context) (int, error) {
	dbCtx := context.WithoutCancel(ctx)

	var (
		published int
		pubErr    error
	)
	err := p.tx.WithTx(dbCtx, func(tx pgx.Tx) error {
		locked, err := p.repo.TryLockTx(dbCtx, tx)
		if err != nil || !locked {
			return err
		}
		batch, err := p.repo.FetchUnpublishedTx(dbCtx, tx, p.opts.BatchSize)
		if err != nil {
			return err
		}

		for _, rec := range batch {
			sendCtx, cancel := context.WithTimeout(ctx, p.opts.PublishTimeout)
			sendErr := p.pub.Publish(sendCtx, rec.Topic, rec.Key, json.RawMessage(rec.Payload))
			cancel()
			if sendErr != nil {
				pubErr = fmt.Errorf("publish outbox event %d (attempt %d): %w", rec.ID, rec.Attempts+1, sendErr)
				return p.repo.RecordFailureTx(dbCtx, tx, rec.ID)
			}
			if err := p.repo.MarkPublishedTx(dbCtx, tx, rec.ID); err != nil {
				return err
			}
			published++
		}
		return nil
	})
	if err != nil {
		return published, err
	}
	return published, pubErr
}

func (p *Publisher) purgeIfDue(ctx context.Context) {
	now := time.Now()
	if now.Before(p.nextGC) {
		return
	}
	p.nextGC = now.Add(p.opts.PurgeEvery)

	deleted, err := p.repo.DeletePublishedBefore(context.WithoutCancel(ctx), p.opts.Retention)
	if err != nil {
		p.log.Error("Outbox purge failed", zap.Error(err))
		return
	}
	if deleted > 0 {
		p.log.Info("Outbox purged", zap.Int64("deleted", deleted))
	}
}

func nextBackoff(cur, max time.Duration) time.Duration {
	if cur == 0 {
		return time.Second
	}
	if cur *= 2; cur > max {
		return max
	}
	return cur
}
