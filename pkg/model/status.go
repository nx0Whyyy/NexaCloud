package model

// InstanceStatus represents the lifecycle state of a Minecraft instance.
type InstanceStatus string

const (
	InstanceCreating     InstanceStatus = "CREATING"
	InstanceStarting     InstanceStatus = "STARTING"
	InstanceReady        InstanceStatus = "READY"
	InstanceActive       InstanceStatus = "ACTIVE"
	InstanceDraining     InstanceStatus = "DRAINING"
	InstanceMaintenance  InstanceStatus = "MAINTENANCE"
	InstanceStopping     InstanceStatus = "STOPPING"
	InstanceStopped      InstanceStatus = "STOPPED"
	InstanceCrashed      InstanceStatus = "CRASHED"
	InstanceSleeping     InstanceStatus = "SLEEPING"
	InstanceQuarantined  InstanceStatus = "QUARANTINED"
)

// NodeStatus represents the status of a physical/virtual node.
type NodeStatus string

const (
	NodeOnline    NodeStatus = "ONLINE"
	NodeOffline   NodeStatus = "OFFLINE"
	NodeDraining  NodeStatus = "DRAINING"
	NodeQuarantined NodeStatus = "QUARANTINED"
)

// PulseStatus is the aggregated health score.
type PulseStatus string

const (
	PulseHealthy   PulseStatus = "HEALTHY"
	PulseDegraded  PulseStatus = "DEGRADED"
	PulseCritical  PulseStatus = "CRITICAL"
	PulseOffline   PulseStatus = "OFFLINE"
)

// AutopilotMode controls automated behavior.
type AutopilotMode string

const (
	AutopilotOff     AutopilotMode = "OFF"
	AutopilotSuggest AutopilotMode = "SUGGEST"
	AutopilotSafe    AutopilotMode = "SAFE"
	AutopilotFull    AutopilotMode = "FULL"
)

// ServiceType classifies a service.
type ServiceType string

const (
	ServiceTypeMinecraft  ServiceType = "minecraft"
	ServiceTypeProxy      ServiceType = "proxy"
	ServiceTypeDatabase   ServiceType = "database"
	ServiceTypeCache      ServiceType = "cache"
	ServiceTypeLimbo      ServiceType = "limbo"
	ServiceTypeCustom     ServiceType = "custom"
)

// SoftwareType identifies the Minecraft software.
type SoftwareType string

const (
	SoftwarePaper     SoftwareType = "paper"
	SoftwareFolia     SoftwareType = "folia"
	SoftwareVelocity  SoftwareType = "velocity"
	SoftwareRedis     SoftwareType = "redis"
	SoftwareMariaDB   SoftwareType = "mariadb"
)

// DeploymentStrategy defines how a deployment is rolled out.
type DeploymentStrategy string

const (
	StrategyAllAtOnce DeploymentStrategy = "all_at_once"
	StrategyRolling   DeploymentStrategy = "rolling"
	StrategyCanary    DeploymentStrategy = "canary"
	StrategyScheduled DeploymentStrategy = "scheduled"
)

// DeploymentStatus tracks rollout progress.
type DeploymentStatus string

const (
	DeploymentPending  DeploymentStatus = "PENDING"
	DeploymentRunning  DeploymentStatus = "RUNNING"
	DeploymentCompleted DeploymentStatus = "COMPLETED"
	DeploymentFailed   DeploymentStatus = "FAILED"
	DeploymentRolledBack DeploymentStatus = "ROLLED_BACK"
)

// BackupType classifies a backup.
type BackupType string

const (
	BackupFull    BackupType = "full"
	BackupWorld   BackupType = "world"
	BackupConfig  BackupType = "config"
	BackupPlugin  BackupType = "plugin"
	BackupDatabase BackupType = "database"
)

// BackupStatus tracks backup progress.
type BackupStatus string

const (
	BackupPending   BackupStatus = "pending"
	BackupRunning   BackupStatus = "running"
	BackupCompleted BackupStatus = "completed"
	BackupFailed    BackupStatus = "failed"
)
