package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	APIPort           int
	NATSUrl           string
	DBHost            string
	DBPort            int
	DBUser            string
	DBPassword        string
	DBName            string
	RedisHost         string
	RedisPort         int
	AdminUsername     string
	AdminEmail        string
	AdminPassword     string
	SMTPHost          string
	SMTPPort          int
	SMTPUsername      string
	SMTPPassword      string
	SMTPFrom          string
	SMTPFromName      string
	PublicURL         string
	ReconcileInterval time.Duration
}

func Load() (*Config, error) {
	c := &Config{
		APIPort:           getEnvInt("API_PORT", 8080),
		NATSUrl:           getEnv("NATS_URL", "nats://localhost:4222"),
		DBHost:            getEnv("DB_HOST", "localhost"),
		DBPort:            getEnvInt("DB_PORT", 5432),
		DBUser:            getEnv("DB_USER", "nexacloud"),
		DBPassword:        getEnv("DB_PASSWORD", "nexacloud"),
		DBName:            getEnv("DB_NAME", "nexacloud"),
		RedisHost:         getEnv("REDIS_HOST", "localhost"),
		RedisPort:         getEnvInt("REDIS_PORT", 6379),
		AdminUsername:     getEnv("NEXA_ADMIN_USERNAME", ""),
		AdminEmail:        getEnv("NEXA_ADMIN_EMAIL", ""),
		AdminPassword:     getEnv("NEXA_ADMIN_PASSWORD", ""),
		SMTPHost:          getEnv("SMTP_HOST", ""),
		SMTPPort:          getEnvInt("SMTP_PORT", 587),
		SMTPUsername:      getEnv("SMTP_USERNAME", ""),
		SMTPPassword:      getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:          getEnv("SMTP_FROM", "no-reply@nexastudio.dev"),
		SMTPFromName:      getEnv("SMTP_FROM_NAME", "NexaCloud"),
		PublicURL:         getEnv("PUBLIC_URL", "https://cloud.nexastudio.dev"),
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
