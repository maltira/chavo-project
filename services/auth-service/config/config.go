package config

import (
	"errors"
	"os"
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

	// SMTP
	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:         getEnv("ENV", "development"),
		Port:        getEnv("AUTH_PORT", "8001"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379/0"),
		JWTSecret:   os.Getenv("JWT_SECRET"),

		AccessTokenDuration:  parseDuration("ACCESS_TOKEN_DURATION", 10*time.Minute),
		RefreshTokenDuration: parseDuration("REFRESH_TOKEN_DURATION", 7*24*time.Hour),
		VerificationTTL:      parseDuration("VERIFICATION_TTL", 15*time.Minute),

		FrontendURL: os.Getenv("FRONTEND_URL"),

		SMTPHost: os.Getenv("SMTP_HOST"),
		SMTPPort: os.Getenv("SMTP_PORT"),
		SMTPUser: os.Getenv("SMTP_USER"),
		SMTPPass: os.Getenv("SMTP_PASS"),
	}

	if cfg.JWTSecret == "" {
		return nil, errors.New("JWT_SECRET is required")
	}
	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASSE_URL is required")
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
