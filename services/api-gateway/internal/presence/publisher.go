package presence

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

const Topic = "presence-events"

// KafkaPublisher пишет асинхронно: переходы не ждут подтверждения брокера. Ошибки отправки только логируются —
// потерянный user.offline оставит last_seen_at прежним до следующего выхода пользователя.
type KafkaPublisher struct {
	w *kafka.Writer
}

func NewKafkaPublisher(brokers []string, log *zap.Logger) *KafkaPublisher {
	return &KafkaPublisher{w: &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        Topic,
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireAll,
		Async:        true,
		BatchTimeout: 50 * time.Millisecond,
		Completion: func(msgs []kafka.Message, err error) {
			if err != nil {
				log.Error("presence events not delivered to Kafka", zap.Int("count", len(msgs)), zap.Error(err))
			}
		},
	}}
}

func (p *KafkaPublisher) Publish(ctx context.Context, key string, value []byte) error {
	return p.w.WriteMessages(ctx, kafka.Message{Key: []byte(key), Value: value})
}

// Close дожидается отправки накопленных сообщений.
func (p *KafkaPublisher) Close() error {
	return p.w.Close()
}
