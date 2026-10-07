// Package auth проверяет access token: подпись и срок JWT плюс живость сессии в Redis.
package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// ErrUnauthorized — токен отсутствует, невалиден, истёк или сессия отозвана.
var ErrUnauthorized = errors.New("unauthorized")

// sessionKeyPrefix — ключ auth-service: ws:session:<sid> = user_id, удаляется при отзыве сессии.
const sessionKeyPrefix = "ws:session:"

// Sessions возвращает владельца живой сессии или "" если её нет.
type Sessions interface {
	Owner(ctx context.Context, sid string) (string, error)
}

type Verifier struct {
	secret   []byte
	sessions Sessions
}

func NewVerifier(secret string, sessions Sessions) *Verifier {
	return &Verifier{secret: []byte(secret), sessions: sessions}
}

type claims struct {
	SID string `json:"sid"`
	jwt.RegisteredClaims
}

// Verify разбирает заголовок Authorization; ошибка, отличная от ErrUnauthorized, — недоступность Redis.
func (v *Verifier) Verify(ctx context.Context, header string) (userID, sid string, err error) {
	raw, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || raw == "" {
		return "", "", ErrUnauthorized
	}

	var c claims
	_, err = jwt.ParseWithClaims(raw, &c, func(*jwt.Token) (any, error) { return v.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return "", "", ErrUnauthorized
	}
	if _, err = uuid.Parse(c.Subject); err != nil {
		return "", "", ErrUnauthorized
	}
	if _, err = uuid.Parse(c.SID); err != nil {
		return "", "", ErrUnauthorized
	}

	owner, err := v.sessions.Owner(ctx, c.SID)
	if err != nil {
		return "", "", err
	}
	if owner != c.Subject {
		return "", "", ErrUnauthorized
	}
	return c.Subject, c.SID, nil
}

type redisSessions struct {
	rdb *redis.Client
}

func NewRedisSessions(rdb *redis.Client) Sessions {
	return &redisSessions{rdb: rdb}
}

func (s *redisSessions) Owner(ctx context.Context, sid string) (string, error) {
	owner, err := s.rdb.Get(ctx, sessionKeyPrefix+sid).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	return owner, err
}
