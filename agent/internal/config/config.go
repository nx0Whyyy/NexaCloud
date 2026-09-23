package config

import (
	"os"
	"strconv"
)

type Config struct {
	NodeID     string
	NATSUrl    string
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
	DockerHost string
}

func Load() (*Config, error) {
	return &Config{
		NodeID:     getEnv("NEXA_NODE_ID", "unknown"),
		NATSUrl:    getEnv("NATS_URL", "nats://localhost:4222"),
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnvInt("DB_PORT", 5432),
		DBUser:     getEnv("DB_USER", "nexacloud"),
		DBPassword: getEnv("DB_PASSWORD", "nexacloud"),
		DBName:     getEnv("DB_NAME", "nexacloud"),
		DockerHost: getEnv("DOCKER_HOST", "unix:///var/run/docker.sock"),
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
