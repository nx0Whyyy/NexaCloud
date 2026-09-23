package config

import (
	"os"
	"strconv"
)

type Config struct {
	APIUrl    string
	NATSUrl   string
}

func Load() (*Config, error) {
	return &Config{
		APIUrl:  getEnv("NEXA_API_URL", "http://localhost:8080"),
		NATSUrl: getEnv("NATS_URL", "nats://localhost:4222"),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func _() {
	_ = strconv.Itoa
}
