// Package profile проверяет, что у пользователя есть профиль: без него доступны только вход и создание профиля.
package profile

import (
	"context"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	userv1 "github.com/maltira/chavo-project-backend/proto/gen/go/user/v1"
)

const keyPrefix = "profile:"

// Cache хранит только положительный ответ: профиль не исчезает, кроме удаления аккаунта.
type Cache interface {
	Has(ctx context.Context, userID string) (bool, error)
	Mark(ctx context.Context, userID string) error
	Forget(ctx context.Context, userID string) error
}

type Gate struct {
	cache Cache
	users userv1.UserInternalServiceClient
	log   *zap.Logger
}

func NewGate(cache Cache, users userv1.UserInternalServiceClient, log *zap.Logger) *Gate {
	return &Gate{cache: cache, users: users, log: log}
}

// Exists: сначала кеш, при промахе — user-service; ошибка — только от user-service.
func (g *Gate) Exists(ctx context.Context, userID string) (bool, error) {
	ok, err := g.cache.Has(ctx, userID)
	if err != nil {
		g.log.Warn("profile cache read failed", zap.Error(err))
	}
	if ok {
		return true, nil
	}

	resp, err := g.users.UserExists(ctx, &userv1.UserExistsRequest{UserId: userID})
	if err != nil {
		return false, err
	}
	if resp.GetExists() {
		g.Created(ctx, userID)
	}
	return resp.GetExists(), nil
}

// Created запоминает профиль сразу после его создания, без лишнего запроса в user-service.
func (g *Gate) Created(ctx context.Context, userID string) {
	if err := g.cache.Mark(ctx, userID); err != nil {
		g.log.Warn("profile cache write failed", zap.Error(err))
	}
}

func (g *Gate) Deleted(ctx context.Context, userID string) error {
	return g.cache.Forget(ctx, userID)
}

type redisCache struct {
	rdb *redis.Client
}

func NewRedisCache(rdb *redis.Client) Cache {
	return &redisCache{rdb: rdb}
}

func (c *redisCache) Has(ctx context.Context, userID string) (bool, error) {
	n, err := c.rdb.Exists(ctx, keyPrefix+userID).Result()
	return n == 1, err
}

func (c *redisCache) Mark(ctx context.Context, userID string) error {
	return c.rdb.Set(ctx, keyPrefix+userID, 1, 0).Err()
}

func (c *redisCache) Forget(ctx context.Context, userID string) error {
	return c.rdb.Del(ctx, keyPrefix+userID).Err()
}
