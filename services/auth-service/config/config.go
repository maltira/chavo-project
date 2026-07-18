package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	Env  string
	Port string

	// Database
	AuthDBDSN string

	// Redis
	RedisURL string

	// JWT
	JWTSecret            string
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration

	// Password reset token
	PassTokenDuration time.Duration

	// Frontend
	FrontendURL string

	// SMTP
	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:                  getEnv("ENV", "development"),
		Port:                 getEnv("AUTH_PORT", "8001"),
		AuthDBDSN:            os.Getenv("AUTH_DB_DSN"),
		RedisURL:             getEnv("REDIS_URL", "redis://localhost:6379/0"),
		JWTSecret:            os.Getenv("JWT_SECRET"),
		AccessTokenDuration:  parseDuration("ACCESS_TOKEN_DURATION", 15*time.Minute),
		RefreshTokenDuration: parseDuration("REFRESH_TOKEN_DURATION", 7*24*time.Hour),
		PassTokenDuration:    parseDuration("PASS_TOKEN_DURATION", 15*time.Minute),
		FrontendURL:          getEnv("FRONTEND_URL", "http://localhost:3000"),
		SMTPHost:             os.Getenv("SMTP_HOST"),
		SMTPPort:             getEnv("SMTP_PORT", "587"),
		SMTPUser:             os.Getenv("SMTP_USER"),
		SMTPPass:             os.Getenv("SMTP_PASS"),
	}

	if cfg.AuthDBDSN == "" {
		return nil, fmt.Errorf("AUTH_DB_DSN is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

func (c *Config) IsProduction() bool {
	return c.Env == "production"
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
