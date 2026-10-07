package hub

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// userKey — ws:user:<user_id> = set connection_id; user-service считает онлайн по SCARD.
const userKey = "ws:user:"

type RedisRegistry struct {
	rdb *redis.Client
}

func NewRedisRegistry(rdb *redis.Client) *RedisRegistry {
	return &RedisRegistry{rdb: rdb}
}

func (r *RedisRegistry) Add(ctx context.Context, userID, connID string) error {
	return r.rdb.SAdd(ctx, userKey+userID, connID).Err()
}

func (r *RedisRegistry) Remove(ctx context.Context, userID, connID string) error {
	return r.rdb.SRem(ctx, userKey+userID, connID).Err()
}

// Reset удаляет все ws:user:* при старте: Gateway один, и после его остановки живых соединений не осталось.
func (r *RedisRegistry) Reset(ctx context.Context) (int, error) {
	var deleted int
	iter := r.rdb.Scan(ctx, 0, userKey+"*", 500).Iterator()
	batch := make([]string, 0, 500)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		n, err := r.rdb.Del(ctx, batch...).Result()
		deleted += int(n)
		batch = batch[:0]
		return err
	}
	for iter.Next(ctx) {
		batch = append(batch, iter.Val())
		if len(batch) == cap(batch) {
			if err := flush(); err != nil {
				return deleted, err
			}
		}
	}
	if err := iter.Err(); err != nil {
		return deleted, err
	}
	return deleted, flush()
}
