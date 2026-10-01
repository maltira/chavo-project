package config

import (
	"errors"
	"os"
	"strings"
	"time"
)

type Config struct {
	Env         string
	Port        string
	DatabaseURL string
	RedisURL    string

	JWTSecret            string
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration
	VerificationTTL      time.Duration

	FrontendURL string

	// Kafka
	KafkaBrokers []string

	// SMTP
	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:         getEnv("ENV", "development"),
		Port:        getEnv("PORT", "8001"),
		DatabaseURL: getEnv("DATABASE_URL", ""),
		RedisURL:    getEnv("REDIS_URL", ""),
		JWTSecret:   getEnv("JWT_SECRET", ""),

		AccessTokenDuration:  parseDuration("ACCESS_TOKEN_DURATION", 10*time.Minute),
		RefreshTokenDuration: parseDuration("REFRESH_TOKEN_DURATION", 7*24*time.Hour),
		VerificationTTL:      parseDuration("VERIFICATION_TTL", 15*time.Minute),

		FrontendURL: getEnv("FRONTEND_URL", ""),

		KafkaBrokers: parseStringSlice("KAFKA_BROKERS", []string{"kafka:29092"}),

		SMTPHost: getEnv("SMTP_HOST", ""),
		SMTPPort: getEnv("SMTP_PORT", ""),
		SMTPUser: getEnv("SMTP_USER", ""),
		SMTPPass: getEnv("SMTP_PASS", ""),
	}

	if cfg.JWTSecret == "" {
		return nil, errors.New("JWT_SECRET is required")
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

func parseDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
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
