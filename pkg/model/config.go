package model

// ConfigScope defines the scope of a configuration entry.
type ConfigScope string

const (
	ConfigScopeGlobal  ConfigScope = "global"
	ConfigScopeService ConfigScope = "service"
	ConfigScopeInstance ConfigScope = "instance"
)

// VersionedConfig stores configuration with version history.
type VersionedConfig struct {
	ID          string `json:"id"`          // e.g. "global/messages.yml"
	Scope       ConfigScope `json:"scope"`
	Content     string `json:"content"`
	Version     int    `json:"version"`
	CreatedBy   string `json:"created_by"`
	CreatedAt   string `json:"created_at"`
}

// NetworkConfig is the top-level declarative network definition.
type NetworkConfig struct {
	Network  NetworkSpec     `json:"network"`
	Proxies  map[string]ProxySpec `json:"proxies"`
	Services map[string]ServiceSpec `json:"services"`
}

type NetworkSpec struct {
	Name string `json:"name"`
}

type ProxySpec struct {
	Type      string `json:"type"`        // "velocity"
	Instances int    `json:"instances"`
	Min       int    `json:"min"`
	Max       int    `json:"max"`
}

type ServiceSpec struct {
	Type        string `json:"type"`        // paper, folia, velocity-limbo
	Instances   int    `json:"instances"`
	Memory      string `json:"memory"`
	AutoScale   bool   `json:"autoScale"`
}

// CommandRequest is a command sent from the control plane to an agent.
type CommandRequest struct {
	ID          string            `json:"id"`          // unique command ID
	Command     string            `json:"command"`     // e.g. START_INSTANCE
	NodeID      string            `json:"node_id"`
	InstanceID  string            `json:"instance_id,omitempty"`
	Params      map[string]string `json:"params,omitempty"`
	ReplyTo     string            `json:"reply_to,omitempty"`
}

// CommandResponse is the result of a command execution.
type CommandResponse struct {
	ID          string `json:"id"`
	Success     bool   `json:"success"`
	Error       string `json:"error,omitempty"`
	Result      string `json:"result,omitempty"`
}
