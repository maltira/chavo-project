package config

import (
	"errors"
	"os"
	"time"
)

type Config struct {
	Env      string
	AuthPort string
	UserPort string

	// Database
	AuthDBDSN string

	// Redis
	RedisURL string

	// JWT
	JWTSecret            string
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration

	// Password reset token
	VerificationTTL time.Duration

	// Frontend
	FrontendURL string

	// SMTP
	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string

	// Internal service communication
	InternalSecret string

	// Inter-service URLs
	UserServiceURL string
}

func Load() (*Config, error) {
	userPort := getEnv("USER_PORT", "8002")
	cfg := &Config{
		Env:       getEnv("ENV", "development"),
		AuthPort:  getEnv("AUTH_PORT", "8001"),
		UserPort:  userPort,
		AuthDBDSN: os.Getenv("AUTH_DB_DSN"),
		RedisURL:  getEnv("REDIS_URL", "redis://localhost:6379/0"),
		JWTSecret: os.Getenv("JWT_SECRET"),

		AccessTokenDuration:  parseDuration("ACCESS_TOKEN_DURATION", 15*time.Minute),
		RefreshTokenDuration: parseDuration("REFRESH_TOKEN_DURATION", 7*24*time.Hour),
		VerificationTTL:      parseDuration("VERIFICATION_TTL", 15*time.Minute),

		FrontendURL: os.Getenv("FRONTEND_URL"),

		SMTPHost: os.Getenv("SMTP_HOST"),
		SMTPPort: getEnv("SMTP_PORT", "587"),
		SMTPUser: os.Getenv("SMTP_USER"),
		SMTPPass: os.Getenv("SMTP_PASS"),

		InternalSecret: os.Getenv("INTERNAL_SECRET"),

		// USER_SERVICE_URL позволяет переопределить адрес user-service.
		// По умолчанию используется имя контейнера в Docker-сети.
		UserServiceURL: getEnv("USER_SERVICE_URL", "http://chavo-user:"+userPort),
	}

	if cfg.JWTSecret == "" {
		return nil, errors.New("JWT_SECRET is required")
	}
	if cfg.AuthDBDSN == "" {
		return nil, errors.New("AUTH_DB_DSN is required")
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
