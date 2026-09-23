package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	NATSUrl       string
	DBHost        string
	DBPort        int
	DBUser        string
	DBPassword    string
	DBName        string
	APIPort       int
	PollInterval  time.Duration
}

func Load() (*Config, error) {
	return &Config{
		NATSUrl:       getEnv("NATS_URL", "nats://localhost:4222"),
		DBHost:        getEnv("DB_HOST", "localhost"),
		DBPort:       getEnvInt("DB_PORT", 5432),
		DBUser:        getEnv("DB_USER", "nexacloud"),
		DBPassword:    getEnv("DB_PASSWORD", "nexacloud"),
		DBName:        getEnv("DB_NAME", "nexacloud"),
		APIPort:       getEnvInt("API_PORT", 8081),
		PollInterval:  getEnvDuration("POLL_INTERVAL", 5*time.Second),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
