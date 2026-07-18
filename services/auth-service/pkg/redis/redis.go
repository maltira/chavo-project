package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func NewClient(ctx context.Context, redisURL string, log *zap.Logger) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	if err = client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	log.Info("Redis connection established")
	return client, nil
}

func Close(client *redis.Client, log *zap.Logger) {
	if client == nil {
		return
	}
	if err := client.Close(); err != nil {
		log.Error("Failed to close Redis", zap.Error(err))
		return
	}
	log.Info("Redis connection closed")
}
