package config

import (
	"errors"
	"os"
	"strings"
)

// Config содержит все параметры конфигурации user-service.
type Config struct {
	Env          string
	Port         string
	DatabaseURL  string
	RedisURL     string
	KafkaBrokers []string
}

// Load читает переменные окружения и возвращает Config.
func Load() (*Config, error) {

	cfg := &Config{
		Env:          getEnv("ENV", "development"),
		Port:         getEnv("PORT", "8002"),
		DatabaseURL:  getEnv("DATABASE_URL", ""),
		RedisURL:     getEnv("REDIS_URL", ""),
		KafkaBrokers: parseStringSlice("KAFKA_BROKERS", []string{"kafka:29092"}),
	}

	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	if cfg.RedisURL == "" {
		return nil, errors.New("REDIS_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseStringSlice(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	res := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			res = append(res, trimmed)
		}
	}
	if len(res) == 0 {
		return fallback
	}
	return res
}
