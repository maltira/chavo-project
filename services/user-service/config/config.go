package config

import (
	"errors"
	"os"
	"strconv"
)

// Config содержит все параметры конфигурации user-service.
type Config struct {
	Env            string
	UserPort       string
	UserDBDSN      string
	RedisURL       string
	InternalSecret string
}

// Load читает переменные окружения и возвращает Config.
func Load() (*Config, error) {
	port := os.Getenv("USER_PORT")
	if port == "" {
		port = "8002"
	}
	if _, err := strconv.Atoi(port); err != nil {
		return nil, errors.New("USER_PORT must be a valid port number")
	}

	dsn := os.Getenv("USER_DB_DSN")
	if dsn == "" {
		return nil, errors.New("USER_DB_DSN is required")
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return nil, errors.New("REDIS_URL is required")
	}

	return &Config{
		Env:            os.Getenv("ENV"),
		UserPort:       port,
		UserDBDSN:      dsn,
		RedisURL:       redisURL,
		InternalSecret: os.Getenv("INTERNAL_SECRET"),
	}, nil
}
