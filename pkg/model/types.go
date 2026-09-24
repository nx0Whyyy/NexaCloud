package model

import (
	"time"

	"github.com/google/uuid"
)

// Resources describes compute resource allocation.
type Resources struct {
	Memory    string `json:"memory,omitempty"` // e.g. "12G"
	CPU       int    `json:"cpu,omitempty"`    // cores limit
	CPULimit  string `json:"cpu_limit,omitempty"`
	DiskQuota string `json:"disk_quota,omitempty"`
}

// ResourceUsage describes current resource consumption.
type ResourceUsage struct {
	CPU     float64 `json:"cpu"`     // percentage 0-100
	RAM     float64 `json:"ram"`     // percentage 0-100
	Disk    float64 `json:"disk"`    // percentage 0-100
	Network float64 `json:"network"` // Mbps
}

// Node represents a physical/virtual machine running NexaAgent.
type Node struct {
	ID             uuid.UUID       `json:"id" gorm:"type:uuid;primarykey"`
	OrganizationID *uuid.UUID      `json:"organization_id,omitempty" gorm:"type:uuid;index"`
	NetworkID      *uuid.UUID      `json:"network_id,omitempty" gorm:"type:uuid;index"`
	Name           string          `json:"name" gorm:"unique;size:100"`
	Status         NodeStatus      `json:"status" gorm:"size:20"`
	Labels         Labels          `json:"labels" gorm:"type:jsonb"`
	Resources      Resources       `json:"resources" gorm:"type:jsonb"`
	Usage          ResourceUsage   `json:"usage" gorm:"type:jsonb"`
	AgentVersion   string          `json:"agent_version"`
	LastHeartbeat  time.Time       `json:"last_heartbeat"`
	Containers     []ContainerInfo `json:"containers,omitempty" gorm:"-"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type Labels map[string]string

type ContainerInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image"`
	State string `json:"state"`
}

// Service defines a logical group of instances (e.g. "skyblock", "lobby").
type Service struct {
	ID             uuid.UUID    `json:"id" gorm:"type:uuid;primarykey"`
	Name           string       `json:"name" gorm:"unique;size:100"`
	Type           ServiceType  `json:"type" gorm:"size:20"`
	Software       SoftwareSpec `json:"software" gorm:"type:jsonb"`
	Resources      Resources    `json:"resources" gorm:"type:jsonb"`
	Autoscaling    *Autoscaling `json:"autoscaling,omitempty" gorm:"type:jsonb"`
	Placement      *Placement   `json:"placement,omitempty" gorm:"type:jsonb"`
	Dependencies   []string     `json:"dependencies,omitempty" gorm:"type:text[]"`
	ConfigTemplate string       `json:"config_template,omitempty"`
	BlueprintID    *uuid.UUID   `json:"blueprint_id,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

type SoftwareSpec struct {
	Type    SoftwareType `json:"type"`
	Version string       `json:"version"`
	Java    JavaSpec     `json:"java,omitempty"`
}

type JavaSpec struct {
	Version int `json:"version"` // major version, e.g. 25
}

type Autoscaling struct {
	Minimum            int          `json:"minimum"`
	Maximum            int          `json:"maximum"`
	ScaleUp            *ScalePolicy `json:"scaleUp,omitempty"`
	ScaleDown          *ScalePolicy `json:"scaleDown,omitempty"`
	PlayersPerInstance int          `json:"players_per_instance,omitempty"`
}

type ScalePolicy struct {
	Players  int    `json:"players,omitempty"`   // player count threshold
	CPU      int    `json:"cpu,omitempty"`       // percentage threshold
	MSPT     int    `json:"mspt,omitempty"`      // mspt threshold
	EmptyFor string `json:"empty_for,omitempty"` // e.g. "10m"
}

type Placement struct {
	Region       []string          `json:"region,omitempty"`
	Requirements map[string]string `json:"requirements,omitempty"` // e.g. memory=12G
	Avoid        []string          `json:"avoid,omitempty"`        // node names/roles
	Spread       map[string]string `json:"spread,omitempty"`       // e.g. service=skyblock
	AntSplit     bool              `json:"ant_split,omitempty"`    // never collocate same service
}

// Instance is a running Minecraft server container.
type Instance struct {
	ID             uuid.UUID       `json:"id" gorm:"type:uuid;primarykey"`
	OrganizationID *uuid.UUID      `json:"organization_id,omitempty" gorm:"type:uuid;index"`
	NetworkID      *uuid.UUID      `json:"network_id,omitempty" gorm:"type:uuid;index"`
	Name           string          `json:"name" gorm:"unique;size:100"` // e.g. skyblock-03
	ServiceName    string          `json:"service_name" gorm:"size:100"`
	NodeID         *uuid.UUID      `json:"node_id,omitempty" gorm:"type:uuid"`
	Status         InstanceStatus  `json:"status" gorm:"size:20"`
	Pulse          PulseStatus     `json:"pulse" gorm:"size:20"`
	ContainerID    string          `json:"container_id,omitempty"`
	Address        string          `json:"address,omitempty"` // internal IP:port
	Port           int             `json:"port,omitempty"`
	Resources      Resources       `json:"resources" gorm:"type:jsonb"`
	Metadata       InstanceMeta    `json:"metadata" gorm:"type:jsonb"`
	Health         *InstanceHealth `json:"health,omitempty" gorm:"type:jsonb"`
	PlayerCount    int             `json:"player_count"`
	Version        string          `json:"version,omitempty"` // software version
	Plugins        []string        `json:"plugins,omitempty" gorm:"type:text[]"`
	ConfigVersion  string          `json:"config_version,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	StartedAt      *time.Time      `json:"started_at,omitempty"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type InstanceMeta struct {
	Software SoftwareSpec      `json:"software,omitempty"`
	Java     JavaSpec          `json:"java,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
}

type InstanceHealth struct {
	Heartbeat       bool      `json:"heartbeat"`
	TPS             float64   `json:"tps"`
	MSPT            float64   `json:"mspt"`
	Process         bool      `json:"process"`
	Console         bool      `json:"console"`
	DockerHealth    bool      `json:"docker_health"`
	LastCheck       time.Time `json:"last_check"`
	CrashCount      int       `json:"crash_count"`
	LastCrashReason string    `json:"last_crash_reason,omitempty"`
}

// RegisteredService is a service instance in the registry.
type RegisteredService struct {
	InstanceID   uuid.UUID `json:"instance_id" gorm:"type:uuid;primarykey"`
	ServiceName  string    `json:"service_name" gorm:"size:100"`
	Address      string    `json:"address"`
	Port         int       `json:"port"`
	Ready        bool      `json:"ready"`
	RegisteredAt time.Time `json:"registered_at"`
}

// TimelineEvent records a significant system event.
type TimelineEvent struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primarykey"`
	Actor        string    `json:"actor"`         // user or "system"
	Action       string    `json:"action"`        // "scale", "restart", etc.
	ResourceType string    `json:"resource_type"` // "instance", "node", "service"
	ResourceID   string    `json:"resource_id"`
	Details      string    `json:"details"` // JSON blob
	CreatedAt    time.Time `json:"created_at"`
}

// AuditEntry records an auditable action.
type AuditEntry struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primarykey"`
	Actor        string    `json:"actor"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	ResourceID   string    `json:"resource_id"`
	IPAddress    string    `json:"ip_address,omitempty"`
	Result       string    `json:"result"`  // success, failure
	Details      string    `json:"details"` // JSON blob
	CreatedAt    time.Time `json:"created_at"`
}
