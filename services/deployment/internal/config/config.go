package config

import (
	"os"
)

type Config struct {
	NATSUrl string
}

func Load() (*Config, error) {
	return &Config{
		NATSUrl: getEnv("NATS_URL", "nats://localhost:4222"),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
