package config

import (
	"os"
	"time"
)

type Config struct {
	NATSUrl            string
	PlannerInterval    time.Duration
}

func Load() (*Config, error) {
	return &Config{
		NATSUrl:   getEnv("NATS_URL", "nats://localhost:4222"),
		PlannerInterval: getEnvDuration("PLANNER_INTERVAL", 1*time.Second),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
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
