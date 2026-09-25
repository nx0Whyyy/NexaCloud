package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	ControlPlaneURL  string
	StatePath        string
	NodeName         string
	Interval         time.Duration
	MinecraftAddress string
}

type State struct {
	NodeID      string `json:"node_id"`
	AgentToken  string `json:"agent_token"`
	Fingerprint string `json:"fingerprint"`
}

func Load() (*Config, error) {
	hostname, _ := os.Hostname()
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	interval, err := time.ParseDuration(getEnv("NEXA_HEARTBEAT_INTERVAL", "10s"))
	if err != nil || interval < 5*time.Second {
		return nil, fmt.Errorf("NEXA_HEARTBEAT_INTERVAL must be at least 5s")
	}
	return &Config{ControlPlaneURL: getEnv("NEXA_CONTROL_PLANE_URL", "https://cloud.nexastudio.dev"), StatePath: getEnv("NEXA_STATE_PATH", filepath.Join(home, ".nexacloud", "agent.json")), NodeName: getEnv("NEXA_NODE_NAME", hostname), Interval: interval, MinecraftAddress: getEnv("NEXA_MINECRAFT_ADDRESS", "127.0.0.1:25565")}, nil
}

func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	if state.NodeID == "" || state.AgentToken == "" {
		return nil, fmt.Errorf("agent state is incomplete")
	}
	return &state, nil
}

func SaveState(path string, state State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
