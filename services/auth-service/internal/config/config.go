package config

import (
	"os"
)

type Config struct {
	Env         string
	Port        string
	AUTH_DB_DSN string
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:         os.Getenv("ENV"),
		Port:        os.Getenv("AUTH_PORT"),
		AUTH_DB_DSN: os.Getenv("AUTH_DB_DSN"),
	}

	return cfg, nil
}

func (c *Config) IsProduction() bool {
	return c.Env == "production"
}
