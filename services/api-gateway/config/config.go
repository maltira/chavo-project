package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Env       string
	Port      string
	JWTSecret string
	RedisURL  string

	AuthAddr         string
	UserAddr         string
	ConversationAddr string
	// GRPCTimeout — дедлайн каждого вызова сервиса.
	GRPCTimeout time.Duration

	// TrustedProxies — IP/CIDR прокси (nginx), чьим X-Forwarded-For/Proto можно верить.
	TrustedProxies []string

	// FrontendOrigin — единственный Origin, с которого принимаются WebSocket-подключения.
	FrontendOrigin string
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:              getEnv("ENV", "development"),
		Port:             getEnv("PORT", "8000"),
		JWTSecret:        os.Getenv("JWT_SECRET"),
		RedisURL:         os.Getenv("REDIS_URL"),
		AuthAddr:         os.Getenv("AUTH_SERVICE_ADDR"),
		UserAddr:         os.Getenv("USER_SERVICE_ADDR"),
		ConversationAddr: os.Getenv("CONVERSATION_SERVICE_ADDR"),
		TrustedProxies:   parseList(os.Getenv("TRUSTED_PROXIES")),
		FrontendOrigin:   os.Getenv("FRONTEND_ORIGIN"),
	}

	timeout, err := time.ParseDuration(getEnv("GRPC_TIMEOUT", "5s"))
	if err != nil || timeout <= 0 {
		return nil, fmt.Errorf("invalid GRPC_TIMEOUT")
	}
	cfg.GRPCTimeout = timeout

	required := map[string]string{
		"JWT_SECRET":                cfg.JWTSecret,
		"REDIS_URL":                 cfg.RedisURL,
		"AUTH_SERVICE_ADDR":         cfg.AuthAddr,
		"USER_SERVICE_ADDR":         cfg.UserAddr,
		"CONVERSATION_SERVICE_ADDR": cfg.ConversationAddr,
		"FRONTEND_ORIGIN":           cfg.FrontendOrigin,
	}
	for name, v := range required {
		if v == "" {
			return nil, errors.New(name + " is required")
		}
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseList(v string) []string {
	var res []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			res = append(res, p)
		}
	}
	return res
}
