package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Env         string
	Port        string
	DatabaseURL string

	UserServiceURL string

	// EncryptionKey - 32-байтный ключ AES-256 для шифрования сообщений
	EncryptionKey []byte

	// Kafka
	KafkaBrokers []string
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:         getEnv("ENV", "development"),
		Port:        getEnv("PORT", "8003"),
		DatabaseURL: getEnv("DATABASE_URL", ""),

		UserServiceURL: getEnv("USER_SERVICE_URL", "http://user-service:8002"),

		KafkaBrokers: parseStringSlice("KAFKA_BROKERS", []string{"kafka:29092"}),
	}

	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}

	key, err := base64.StdEncoding.DecodeString(os.Getenv("MESSAGE_ENCRYPTION_KEY"))
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("MESSAGE_ENCRYPTION_KEY must be a base64-encoded 32-byte key")
	}
	cfg.EncryptionKey = key

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
