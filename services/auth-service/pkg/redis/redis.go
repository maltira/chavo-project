package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func NewClient(ctx context.Context, redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	if err = client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return client, nil
}

func Close(client *redis.Client) {
	if client == nil {
		return
	}
	client.Close()
}
