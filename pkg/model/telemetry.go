package model

import (
	"time"

	"github.com/google/uuid"
)

// TelemetryPacket is sent by agents/nexalink to the control plane.
type TelemetryPacket struct {
	Source     string              `json:"source"` // "node" or "instance"
	NodeID     *string             `json:"node_id,omitempty"`
	InstanceID *string             `json:"instance_id,omitempty"`
	Usage      *ResourceUsage      `json:"usage,omitempty"`
	Minecraft  *MinecraftTelemetry `json:"minecraft,omitempty"`
	System     *SystemTelemetry    `json:"system,omitempty"`
	Timestamp  time.Time           `json:"timestamp"`
}

// MinecraftTelemetry is collected by NexaLink from a Minecraft instance.
type MinecraftTelemetry struct {
	TPS           float64       `json:"tps"`
	MSPT          float64       `json:"mspt"`
	Players       int           `json:"players"`
	MaxPlayers    int           `json:"max_players"`
	Chunks        int           `json:"chunks"`
	Entities      int           `json:"entities"`
	Worlds        []string      `json:"worlds"`
	Plugins       []PluginState `json:"plugins"`
	ShutdownState string        `json:"shutdown_state,omitempty"` // "none", "preparing", "draining", "ready"
}

type PluginState struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Enabled bool   `json:"enabled"`
}

// SystemTelemetry is collected by the agent from the host.
type SystemTelemetry struct {
	CPU         float64 `json:"cpu"`         // percentage
	RAM         float64 `json:"ram"`         // percentage
	Disk        float64 `json:"disk"`        // percentage
	NetworkRx   float64 `json:"network_rx"`  // Mbps
	NetworkTx   float64 `json:"network_tx"`  // Mbps
	Temperature float64 `json:"temperature"` // celsius
	LoadAvg     float64 `json:"load_avg"`    // 1-minute load average
	Uptime      int64   `json:"uptime"`      // seconds
}

// NodeMetrics is aggregated metrics for a node.
type NodeMetrics struct {
	NodeID    uuid.UUID     `json:"node_id"`
	Usage     ResourceUsage `json:"usage"`
	Instances int           `json:"instances"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// GraphPoint is a single point in a time-series graph.
type GraphPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

type AgentHeartbeat struct {
	AgentVersion string              `json:"agent_version"`
	Resources    Resources           `json:"resources"`
	Usage        ResourceUsage       `json:"usage"`
	Containers   []ContainerInfo     `json:"containers"`
	Minecraft    *MinecraftTelemetry `json:"minecraft,omitempty"`
}

type AgentHeartbeatResponse struct {
	Status     string    `json:"status"`
	NodeID     uuid.UUID `json:"node_id"`
	ReceivedAt time.Time `json:"received_at"`
}
