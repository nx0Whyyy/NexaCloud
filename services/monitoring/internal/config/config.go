package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	APIPort       int
	DBHost        string
	DBPort        int
	DBUser        string
	DBPassword    string
	DBName        string
	CheckInterval time.Duration
	GitHubRepo    string
	GitHubToken   string
	Orchestrator  string
	Registry      string
	RedisAddress  string
	NATSAddress   string
}

func Load() *Config {
	return &Config{
		APIPort:       envInt("API_PORT", 8082),
		DBHost:        env("DB_HOST", "localhost"),
		DBPort:        envInt("DB_PORT", 5432),
		DBUser:        env("DB_USER", "nexacloud"),
		DBPassword:    env("DB_PASSWORD", "nexacloud"),
		DBName:        env("DB_NAME", "nexacloud"),
		CheckInterval: envDuration("CHECK_INTERVAL", 30*time.Second),
		GitHubRepo:    env("GITHUB_REPOSITORY", "nx0Whyyy/NexaCloud"),
		GitHubToken:   os.Getenv("GITHUB_TOKEN"),
		Orchestrator:  env("ORCHESTRATOR_URL", "http://orchestrator:8080/healthz"),
		Registry:      env("REGISTRY_URL", "http://registry:8081/api/v1/services"),
		RedisAddress:  env("REDIS_ADDRESS", "redis:6379"),
		NATSAddress:   env("NATS_ADDRESS", "nats:4222"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return value
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return value
}
