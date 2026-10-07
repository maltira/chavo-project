// Package kafka содержит consumer'ы (импортирует service, поэтому отдельно от pkg/kafka).
package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/events"
)

// LastSeenUpdater — часть ProfileService, нужная consumer'у.
type LastSeenUpdater interface {
	UpdateLastSeen(ctx context.Context, userID uuid.UUID, at time.Time) error
}

// PresenceConsumer по user.offline обновляет last_seen_at (обновление идемпотентно и только вперёд).
type PresenceConsumer struct {
	reader  *kafka.Reader
	updater LastSeenUpdater
	log     *zap.Logger
}

func NewPresenceConsumer(brokers []string, updater LastSeenUpdater, log *zap.Logger) *PresenceConsumer {
	return &PresenceConsumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:     brokers,
			GroupID:     "user-service",
			Topic:       events.TopicPresenceEvents,
			StartOffset: kafka.FirstOffset,
			MaxWait:     time.Second,
		}),
		updater: updater,
		log:     log,
	}
}

// Run читает до отмены ctx; offset коммитится только после успешной обработки.
func (c *PresenceConsumer) Run(ctx context.Context) {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
				return
			}
			c.log.Error("Presence consumer fetch failed", zap.Error(err))
			time.Sleep(time.Second)
			continue
		}
		// FetchMessage не возвращает незакоммиченное сообщение повторно, поэтому сбой БД повторяем на месте.
		for {
			err := Handle(ctx, c.updater, msg.Value)
			if err == nil {
				break
			}
			c.log.Error("Presence event handling failed", zap.Error(err))
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil && ctx.Err() == nil {
			c.log.Error("Presence consumer commit failed", zap.Error(err))
		}
	}
}

func (c *PresenceConsumer) Close() error {
	return c.reader.Close()
}

// Handle обрабатывает одно событие; нераспознанные и чужие типы пропускаются без ошибки.
func Handle(ctx context.Context, updater LastSeenUpdater, value []byte) error {
	var ev events.Event[events.PresencePayload]
	if err := json.Unmarshal(value, &ev); err != nil {
		return nil // битое событие повтором не исправить
	}
	if ev.EventType != events.TypeUserOffline || ev.Payload.UserID == uuid.Nil || ev.Payload.At.IsZero() {
		return nil
	}
	return updater.UpdateLastSeen(ctx, ev.Payload.UserID, ev.Payload.At)
}
