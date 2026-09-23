package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	APIPort    int
	NATSUrl   string
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
	RedisHost  string
	RedisPort  int
	ReconcileInterval time.Duration
}

func Load() (*Config, error) {
	c := &Config{
		APIPort:    getEnvInt("API_PORT", 8080),
		NATSUrl:   getEnv("NATS_URL", "nats://localhost:4222"),
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnvInt("DB_PORT", 5432),
		DBUser:     getEnv("DB_USER", "nexacloud"),
		DBPassword: getEnv("DB_PASSWORD", "nexacloud"),
		DBName:     getEnv("DB_NAME", "nexacloud"),
		RedisHost:  getEnv("REDIS_HOST", "localhost"),
		RedisPort:  getEnvInt("REDIS_PORT", 6379),
		ReconcileInterval: getEnvDuration("RECONCILE_INTERVAL", 5*time.Second),
	}
	return c, nil
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
