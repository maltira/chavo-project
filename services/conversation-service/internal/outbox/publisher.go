package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/events"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/repository"
)

// EventPublisher — отправка события в брокер; реализует *kafka.Producer.
type EventPublisher interface {
	Publish(ctx context.Context, topic, key string, event any) error
}

// Decrypter расшифровывает content_enc; реализует *crypto.Cipher.
type Decrypter interface {
	Decrypt(data []byte) ([]byte, error)
}

type Options struct {
	PollInterval   time.Duration // пауза, когда очередь пуста
	BatchSize      int
	PublishTimeout time.Duration // лимит на одну отправку в Kafka
	MaxBackoff     time.Duration // потолок паузы после ошибки
	PurgeEvery     time.Duration
	Retention      time.Duration // сколько хранить опубликованные события (для повторной публикации)
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

// Publisher переносит события из outbox_events в Kafka (at-least-once, порядок по id).
type Publisher struct {
	db     *repository.DB
	repo   repository.OutboxRepository
	pub    EventPublisher
	dec    Decrypter
	log    *zap.Logger
	opts   Options
	nextGC time.Time
}

func NewPublisher(db *repository.DB, repo repository.OutboxRepository, pub EventPublisher, dec Decrypter, log *zap.Logger, opts Options) *Publisher {
	return &Publisher{db: db, repo: repo, pub: pub, dec: dec, log: log, opts: opts}
}

// preparePayload для message-events подменяет content_enc на расшифрованный content.
// Без content_enc (текст вычищен при удалении) и для остальных топиков payload уходит как есть.
func (p *Publisher) preparePayload(rec repository.OutboxRecord) ([]byte, error) {
	if rec.Topic != events.TopicMessageEvents {
		return rec.Payload, nil
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(rec.Payload, &envelope); err != nil {
		return nil, fmt.Errorf("parse envelope: %w", err)
	}
	rawPayload, ok := envelope["payload"]
	if !ok {
		return rec.Payload, nil
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return nil, fmt.Errorf("parse payload: %w", err)
	}
	rawEnc, ok := payload["content_enc"]
	if !ok {
		return rec.Payload, nil
	}

	var enc []byte
	if err := json.Unmarshal(rawEnc, &enc); err != nil {
		return nil, fmt.Errorf("parse content_enc: %w", err)
	}
	plain, err := p.dec.Decrypt(enc)
	if err != nil {
		return nil, err
	}
	content, err := json.Marshal(string(plain))
	if err != nil {
		return nil, fmt.Errorf("marshal content: %w", err)
	}
	delete(payload, "content_enc")
	payload["content"] = content

	if envelope["payload"], err = json.Marshal(payload); err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	return json.Marshal(envelope)
}

// Run крутит публикацию до отмены ctx.
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
			backoff, delay = 0, 0 // очередь не разгребена — сразу следующая пачка
		default:
			backoff = 0
		}

		p.purgeIfDue(ctx)

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

// RunOnce публикует одну пачку и возвращает число отправленных событий.
// При ошибке отправки уже успешные события остаются помеченными, а на проблемном растёт attempts;
// оставшиеся не трогаются, чтобы не нарушить порядок.
func (p *Publisher) RunOnce(ctx context.Context) (int, error) {
	// Фиксация результата в БД не должна срываться из-за остановки сервиса.
	dbCtx := context.WithoutCancel(ctx)

	var (
		published int
		pubErr    error
	)
	err := p.db.WithTx(dbCtx, func(tx repository.DBTX) error {
		locked, err := p.repo.TryLock(dbCtx, tx)
		if err != nil || !locked {
			return err
		}
		batch, err := p.repo.FetchUnpublished(dbCtx, tx, p.opts.BatchSize)
		if err != nil {
			return err
		}

		for _, rec := range batch {
			payload, sendErr := p.preparePayload(rec)
			if sendErr == nil {
				sendCtx, cancel := context.WithTimeout(ctx, p.opts.PublishTimeout)
				sendErr = p.pub.Publish(sendCtx, rec.Topic, rec.Key, json.RawMessage(payload))
				cancel()
			}

			if sendErr != nil {
				pubErr = fmt.Errorf("publish outbox event %d (attempt %d): %w", rec.ID, rec.Attempts+1, sendErr)
				return p.repo.RecordFailure(dbCtx, tx, rec.ID)
			}
			if err := p.repo.MarkPublished(dbCtx, tx, rec.ID); err != nil {
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

	deleted, err := p.repo.DeletePublishedBefore(context.WithoutCancel(ctx), p.db.Q(), p.opts.Retention)
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
