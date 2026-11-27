package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/events"
)

// AccountDeletedHandler - часть GroupService, нужная consumer'у auth-events
type AccountDeletedHandler interface {
	HandleAccountDeleted(ctx context.Context, userID uuid.UUID) error
}

// AuthConsumer обрабатывает user.account_deleted из auth-events
type AuthConsumer struct {
	reader  *kafka.Reader
	handler AccountDeletedHandler
	log     *zap.Logger
}

func NewAuthConsumer(brokers []string, handler AccountDeletedHandler, log *zap.Logger) *AuthConsumer {
	return &AuthConsumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:     brokers,
			GroupID:     "conversation-service-auth",
			Topic:       events.TopicAuthEvents,
			StartOffset: kafka.FirstOffset,
			MaxWait:     time.Second,
		}),
		handler: handler,
		log:     log,
	}
}

// Run читает auth-events до отмены ctx; offset коммитится только после успешной обработки
func (c *AuthConsumer) Run(ctx context.Context) {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
				return
			}
			c.log.Error("Auth consumer fetch failed", zap.Error(err))
			time.Sleep(time.Second)
			continue
		}
		for {
			err := HandleAuthEvent(ctx, c.handler, msg.Value)
			if err == nil {
				break
			}
			c.log.Error("Auth event handling failed", zap.Error(err))
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil && ctx.Err() == nil {
			c.log.Error("Auth consumer commit failed", zap.Error(err))
		}
	}
}

func (c *AuthConsumer) Close() error {
	return c.reader.Close()
}

// HandleAuthEvent обрабатывает одно событие auth-events; прочие типы и битый JSON пропускаются
func HandleAuthEvent(ctx context.Context, handler AccountDeletedHandler, value []byte) error {
	var ev events.Event[events.AccountDeletedPayload]
	if err := json.Unmarshal(value, &ev); err != nil {
		return nil
	}
	if ev.EventType != events.TypeAccountDeleted || ev.Payload.UserID == uuid.Nil {
		return nil
	}
	return handler.HandleAccountDeleted(ctx, ev.Payload.UserID)
}
