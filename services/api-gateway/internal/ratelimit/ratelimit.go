// Package ratelimit — лимиты запросов по пользователю: фиксированное окно в Redis (rl:<rule>:<user>:<window>).
// Лимиты по IP режет nginx; здесь — то, что nginx не видит: кто именно шлёт запросы.
package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Rule struct {
	Name   string
	Limit  int
	Window time.Duration
}

// ParseRule разбирает "<limit>/<window>", например "30/10s" или "300/1m"
func ParseRule(name, spec string) (Rule, error) {
	n, w, ok := strings.Cut(strings.TrimSpace(spec), "/")
	limit, err := strconv.Atoi(n)
	if !ok || err != nil || limit <= 0 {
		return Rule{}, fmt.Errorf("invalid rate limit %q for %s: expected <limit>/<window>", spec, name)
	}
	window, err := time.ParseDuration(w)
	if err != nil || window < time.Second || window%time.Second != 0 {
		return Rule{}, fmt.Errorf("invalid rate limit window %q for %s: whole seconds expected", w, name)
	}
	return Rule{Name: name, Limit: limit, Window: window}, nil
}

// Counter увеличивает счётчик окна и при первом увеличении ставит ему TTL
type Counter interface {
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
}

type Limiter struct {
	counter Counter
	now     func() time.Time
}

func New(counter Counter) *Limiter {
	return &Limiter{counter: counter, now: time.Now}
}

// Allow засчитывает запрос; при превышении retryAfter — время до начала следующего окна.
func (l *Limiter) Allow(ctx context.Context, r Rule, key string) (allowed bool, retryAfter time.Duration, err error) {
	now := l.now()
	sec := int64(r.Window / time.Second)
	window := now.Unix() / sec
	n, err := l.counter.Incr(ctx, "rl:"+r.Name+":"+key+":"+strconv.FormatInt(window, 10), r.Window)
	if err != nil {
		return true, 0, err
	}
	if n <= int64(r.Limit) {
		return true, 0, nil
	}
	return false, time.Unix((window+1)*sec, 0).Sub(now), nil
}

type redisCounter struct {
	rdb *redis.Client
}

func NewRedisCounter(rdb *redis.Client) Counter {
	return &redisCounter{rdb: rdb}
}

func (c *redisCounter) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	pipe := c.rdb.TxPipeline()
	incr := pipe.Incr(ctx, key)
	// Ключ живёт не дольше своего окна; NX — TTL не продлевается каждым запросом.
	pipe.ExpireNX(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}
