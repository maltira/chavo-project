// Package presence определяет онлайн по WS-соединениям, которые Gateway держит в Redis (ws:user:{id}).
package presence

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const userConnectionsKey = "ws:user:"

type Checker interface {
	// Online — есть ли у пользователей хотя бы одно WS-соединение.
	Online(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]bool, error)
}

type redisChecker struct {
	rdb *redis.Client
}

func NewChecker(rdb *redis.Client) Checker {
	return &redisChecker{rdb: rdb}
}

func (c *redisChecker) Online(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	res := make(map[uuid.UUID]bool, len(userIDs))
	if len(userIDs) == 0 {
		return res, nil
	}
	pipe := c.rdb.Pipeline()
	cmds := make([]*redis.IntCmd, len(userIDs))
	for i, id := range userIDs {
		cmds[i] = pipe.SCard(ctx, userConnectionsKey+id.String())
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("presence scard: %w", err)
	}
	for i, id := range userIDs {
		res[id] = cmds[i].Val() > 0
	}
	return res, nil
}
