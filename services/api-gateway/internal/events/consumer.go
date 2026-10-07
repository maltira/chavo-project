package events

import (
	"context"
	"errors"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

const (
	groupID       = "api-gateway"
	handleRetries = 3
)

// Consumer читает топики событий группой api-gateway и передаёт их Router.
type Consumer struct {
	reader *kafka.Reader
	router *Router
	log    *zap.Logger
}

func NewConsumer(brokers []string, router *Router, log *zap.Logger) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:     brokers,
			GroupID:     groupID,
			GroupTopics: Topics,
			// Без сохранённого offset история не нужна: клиенты получают состояние через REST.
			StartOffset:    kafka.LastOffset,
			MaxWait:        500 * time.Millisecond,
			CommitInterval: time.Second,
		}),
		router: router,
		log:    log,
	}
}

// Run читает до отмены ctx. Offset коммитится после обработки (периодически, CommitInterval).
func (c *Consumer) Run(ctx context.Context) {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.log.Error("Kafka fetch failed", zap.Error(err))
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}
		c.handle(ctx, msg)
		if err := c.reader.CommitMessages(ctx, msg); err != nil && ctx.Err() == nil {
			c.log.Error("Kafka commit failed", zap.Error(err))
		}
	}
}

// handle повторяет временные сбои несколько раз и не блокирует доставку остальным: сбой Redis у одного события
// не должен останавливать все чаты.
func (c *Consumer) handle(ctx context.Context, msg kafka.Message) {
	fields := []zap.Field{zap.String("topic", msg.Topic), zap.Int("partition", msg.Partition), zap.Int64("offset", msg.Offset)}
	for attempt := 1; ; attempt++ {
		err := c.router.Handle(ctx, msg.Value)
		switch {
		case err == nil:
			return
		case errors.Is(err, ErrMalformed):
			c.log.Warn("Malformed event skipped", fields...)
			return
		case attempt >= handleRetries:
			c.log.Error("Event dropped after retries", append(fields, zap.Error(err))...)
			return
		}
		c.log.Warn("Event handling failed, retrying", append(fields, zap.Error(err))...)
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
