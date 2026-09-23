# NexaCloud - Architecture Plan

> **Run your Minecraft network like a cloud.**
>
> This document describes the technical architecture, component breakdown, data models, communication patterns, and implementation roadmap for NexaCloud.

---

## 0. Vision & Philosophy

NexaCloud treats a Minecraft network as a single, declarative infrastructure entity. The operator writes desired state; the control plane reconciles actual state toward it using Minecraft-aware logic (NexaLink telemetry, player-aware routing, graceful drain, warm pools, wake-on-join, crash-loop protection, etc.).

**Core principle:** Express what you want. The system decides how to achieve it safely.

---

## 1. High-Level Architecture

```text
                        INTERNET
                            |
                            v
                   +----------------+
                   |  PROXY (Velo)  |
                   |  + NexaProxy   |
                   +----------------+
                            |
                            v
+------------------------------------------------------------------+
|                        NEXACLOUD CORE                            |
|                                                                  |
|  API Gateway     Scheduler      Autoscaler     Deployment       |
|  Orchestrator    Registry       Health Engine   Backup Ctrl      |
|  Secrets Mgr     Event Engine   Metrics         Monitoring       |
|                                                                  |
|  PostgreSQL (state)   Redis (cache)   NATS (messaging)          |
+-----+-------------------+-------------------+--------------------+
      |                   |                   |
      v                   v                   v
  NODE-01             NODE-02             NODE-03
  (Docker)            (Docker)            (Docker)
      |                   |                   |
  NexaAgent           NexaAgent           NexaAgent
      |                   |                   |
  Minecraft           Minecraft           Minecraft
  Instances (+NL)     Instances (+NL)     Instances (+NL)
```

---

## 2. Repository Layout (Monorepo)

```text
nexacloud/
├── apps/
│   ├── dashboard/              # React + TypeScript + Vite + Tailwind
│   └── api/                    # Shared API Gateway (optional, can be in orchestrator)
│
├── services/
│   ├── orchestrator/           # Main reconciliation engine (Go)
│   ├── scheduler/              # Node placement + affinity (Go)
│   ├── autoscaler/             # Scaling policies + warm pools (Go)
│   ├── registry/               # Service discovery + routing (Go)
│   ├── deployment/             # Plugin/config rollouts (Go)
│   ├── backup/                 # Backup engine + consistent snapshot (Go)
│   └── monitoring/             # Health engine + pulse + alerting (Go)
│
├── agent/                      # NexaAgent - VPS daemon (Go)
│
├── minecraft/
│   ├── nexalink-paper/         # Java plugin (Paper)
│   ├── nexalink-folia/         # Java plugin (Folia)
│   └── nexalink-velocity/      # Java plugin (Velocity proxy)
│
├── cli/                        # Go CLI (nexa)
│
├── docker/                     # Dockerfiles + compose files
│   ├── Dockerfile.agent        # Multi-stage: Go agent
│   ├── Dockerfile.service      # Generic Go service
│   └── compose/
│       ├── docker-compose.yml  # Control plane services
│       └── minecraft-service.yml # Example Minecraft instance template
│
├── docs/                       # Specification + architecture docs
│   └── spec.md
│
├── Makefile                    # Build/lint/test targets
├── AGENTS.md
└── go.work                     # Go workspace for all Go modules
```

---

## 3. Technology Stack

### Control Plane (Go)
- **Go 1.22+** (workspace mode)
- **PostgreSQL** — durable state (instances, nodes, configs, timeline, audit log)
- **Redis** — cache, distributed locks, transient state (warm pools, queues)
- **NATS JetStream** — agent command/response, events, heartbeats, telemetry streams
- **gRPC** — internal service-to-service communication
- **HTTP/JSON** — public API (API-first)

### Frontend
- **React 18** + **TypeScript** + **Vite**
- **Tailwind CSS**
- **WebSocket** — real-time updates (state changes, timeline events, telemetry)
- **Chart.js** (or similar) — monitoring graphs

### Minecraft Integration
- **NexaLink** — Java plugins for Paper, Folia, and Velocity
  - Protocol: TCP (gRPC or custom binary over mTLS tunnel through NATS/WebSocket)
  - Telemetry: TPS, MSPT, players, chunks, entities, worlds, plugin state
  - Commands: prepareShutdown(), drainPlayers(), saveWorlds(), lockJoins(), snapshotReady(), healthCheck()

### Agent (Go)
- **Docker SDK** — manage Minecraft instance containers
- **mTLS** — identity via signed machine certificates
- **NATS** — bidirectional communication with control plane
- **gRPC** — control plane → agent commands

### Infrastructure
- **Docker Compose** — local development
- **Docker** — production deployment (single binary + compose)
- **No Kubernetes** — keep it accessible for Minecraft operators

### Tooling
- **golangci-lint** — Go linting
- **golangci-lint + gofmt** — formatting
- **go test** — unit testing
- **go vet** — static analysis
- **ESLint + Prettier** — frontend linting/formatting
- **TypeScript** strict mode — type checking
- **taipan** or **golang-migrate** — database migrations

---

## 4. Communication Patterns

### Control Plane → Agent (Commands)
- **NATS request/reply subjects**: `commands.node.{nodeID}`
- Signed JWTs in NATS headers for auth
- Responses over `commands.node.{nodeID}.reply`

### Agent → Control Plane (Telemetry + Heartbeats)
- **NATS JetStream streams**: `telemetry.heartbeat`, `telemetry.metrics`, `instance.status`
- Agent publishes heartbeats every 10s
- Minecraft instances publish through NexaLink (via agent tunnel)

### Control Plane Internal
- **gRPC** between Go services (orchestrator → scheduler, etc.)
- **PostgreSQL** for durable state
- **Redis** for distributed locks + cache

### Dashboard ↔ Control Plane
- **HTTP/HTTPS** — REST API + WebSocket for real-time events
- JWT-based authentication

---

## 5. Data Model (PostgreSQL)

### Core Entities

```sql
-- Nodes (VPS machines)
nodes:
  id UUID PK
  name STRING UNIQUE
  labels JSONB                -- region=eu, dc=belgium, etc.
  status ENUM (ONLINE, OFFLINE, DRAINING)
  resources: cpu, ram, disk (total)
  usage: cpu, ram, disk (current)
  agent_version STRING
  last_heartbeat TIMESTAMP
  created_at, updated_at

-- Services (logical groups like "skyblock", "lobby")
services:
  id UUID PK
  name STRING UNIQUE
  type ENUM (minecraft, proxy, database, cache, limbo, custom)
  software: { type: paper|folia|velocity|redis|mariadb, version }
  java_version INT
  resources: memory, cpu_limit
  blueprint_id FK
  autoscaling JSONB           -- min/max instances, thresholds
  placement JSONB             -- region, spread, avoid
  dependencies TEXT[]         -- service names this depends on
  config_template STRING
  created_at, updated_at

-- Instances (actual running Minecraft servers)
instances:
  id UUID PK
  name STRING UNIQUE         -- e.g. skyblock-03
  service_id FK → services
  node_id FK → nodes
  status ENUM (CREATING, STARTING, READY, ACTIVE, DRAINING,
              MAINTENANCE, STOPPING, STOPPED, CRASHED, SLEEPING, QUARANTINED)
  container_id STRING
  address STRING             -- internal address:port
  resources: memory, cpu
  metadata JSONB             -- version, plugins, config versions
  health JSONB               -- pulse: HEALTHY|DEGRADED|CRITICAL|OFFLINE
  player_count INT
  created_at, updated_at

-- NexaLink telemetry (last known state per instance)
instance_telemetry:
  instance_id FK → instances
  tps FLOAT
  mspt FLOAT
  players INT
  chunks INT
  entities INT
  worlds JSONB
  plugins_state JSONB
  last_seen TIMESTAMP

-- Service Discovery / Routing
registered_services:
  instance_id FK → instances
  service_name STRING
  address STRING
  port INT
  ready BOOLEAN
  registered_at TIMESTAMP

-- Configurations (versioned)
configs:
  id UUID PK
  scope ENUM (global, service, instance)
  path STRING                 -- e.g. "skyblock/economy.yml"
  content TEXT
  version INT
  created_by STRING
  created_at TIMESTAMP

-- Backups
backups:
  id UUID PK
  instance_id FK → instances
  type ENUM (full, world, config, plugin, database)
  destination STRING          -- local, s3, r2, b2, minio, ftp, sftp
  path STRING
  size BIGINT
  status ENUM (pending, running, completed, failed)
  created_at

-- Deployments
deployments:
  id UUID PK
  service_id FK → services
  plugin_name STRING
  version STRING
  strategy ENUM (all_at_once, rolling, canary)
  canary_percentage INT
  status ENUM (pending, running, completed, failed, rolled_back)
  created_by STRING
  created_at, completed_at

-- Timeline events
timeline_events:
  id UUID PK
  actor STRING                -- user or system
  action STRING               -- "scale", "restart", "deploy", etc.
  resource_type STRING        -- instance, node, service, etc.
  resource_id STRING
  details JSONB
  created_at TIMESTAMP

-- Audit log
audit_log:
  id UUID PK
  actor STRING
  action STRING
  resource_type STRING
  resource_id STRING
  ip_address STRING
  result ENUM (success, failure)
  details JSONB
  created_at TIMESTAMP

-- Secrets (encrypted at rest)
secrets:
  id UUID PK
  key STRING UNIQUE
  value_encrypted STRING
  scope ENUM (global, service, instance)
  rotated_at TIMESTAMP
  created_at TIMESTAMP
```

---

## 6. Component Breakdown

### 6.1 Orchestrator (Core Reconciliation Engine)

**Responsibility:** The brain. Continuously reconciles desired state vs. actual state.

**Module structure:**
```text
services/orchestrator/
├── cmd/
│   └── main.go                 # Entry point
├── internal/
│   ├── reconciler/             # Main reconciliation loop
│   │   ├── service_reconciler.go    # Ensure service instance counts
│   │   ├── node_reconciler.go       # Handle node offline/drain
│   │   └── telemetry_reconciler.go  # Process agent/telemetry updates
│   ├── lifecycle/              # Instance lifecycle management
│   │   ├── create.go
│   │   ├── start.go
│   │   ├── stop.go
│   │   ├── delete.go
│   │   └── restart.go
│   ├── drain/                  # Drain logic (players, health, etc.)
│   │   ├── evacuate.go
│   │   └── wait_for_empty.go
│   ├── shift/                  # Nexa Shift - blue/green instance replacement
│   │   ├── create_next.go
│   │   ├── validate.go
│   │   ├── drain_old.go
│   │   └── replace.go
│   ├── crashloop/              # Crash loop detection + quarantine
│   └── autopilot/              # Autopilot mode: SUGGEST/SAFE/FULL
└── pkg/
    └── model/                  # Shared data types
```

**Reconciliation loop:**
```text
Every 5s:
  1. Load all Services from DB
  2. For each service:
     a. Compare desired instances vs actual ready instances
     b. If deficit: schedule creation (consider autoscaling, warm pools)
     c. If surplus: graceful drain + stop
  3. For each instance:
     a. Check health (heartbeat, NexaLink, Docker health)
     b. If unhealthy > threshold: restart/quarantine
  4. Check crash loops per instance
  5. Reconcile node statuses (offline detection, drain scheduling)
```

### 6.2 Scheduler

**Responsibility:** Decide which node hosts new instances.

**Module structure:**
```text
services/scheduler/
├── cmd/main.go
├── internal/
│   ├── planner/                # Bin-packing / placement scoring
│   │   ├── score.go            # Score nodes based on CPU, RAM, disk, region
│   │   ├── affinity.go         # placement requirements (region, memory)
│   │   ├── anti_affinity.go    # anti-split network (spread instances)
│   │   └── avoid.go            # avoid specific nodes/services
│   ├── queue/                  # Pending placement queue
│   └── allocator/              # Port/resource allocation
```

**Placement algorithm:**
```text
Score(node) = weighted_sum:
  - Resource availability (40%): CPU free, RAM free, disk free
  - Region match (20%): preferred region
  - Anti-affinity (15%): fewer service instances already on node
  - Avoid list (10%): -100 if in avoid list
  - Existing load (10%): fewer total instances
  - Node health (5%): ONLINE best, DRAINING penalized
```

### 6.3 Autoscaler

**Responsibility:** Scale services up/down based on metrics.

**Module structure:**
```text
services/autoscaler/
├── cmd/main.go
├── internal/
│   ├── scaler/                 # Core scaling logic
│   │   ├── scale_up.go
│   │   ├── scale_down.go
│   │   └── warmup_pool.go      # Warm pool management
│   ├── policy/                 # Scale policies (CPU, MSPT, players)
│   └── waker/                  # Wake-on-join for sleeping services
```

### 6.4 Registry

**Responsibility:** Dynamic service discovery. Maintains the routing table.

**Module structure:**
```text
services/registry/
├── cmd/main.go
├── internal/
│   ├── discovery/              # Register/unregister instances
│   │   ├── register.go
│   │   ├── unregister.go
│   │   └── update.go
│   ├── routing/                # Smart routing decisions
│   │   ├── selector.go         # Choose best instance for player
│   │   └── metrics_cache.go    # Cache metrics for routing
│   └── proxy_sync/             # Push updates to NexaProxy
│       └── velocity.go         # Velocity plugin API calls
```

**Routing algorithm:**
```text
select_instance(service):
  candidates = all READY + ACTIVE instances for service
  for each candidate:
    score = weighted:
      - Players (30%): fewer players = better
      - TPS (25%): higher = better
      - MSPT (20%): lower = better
      - CPU usage (15%): lower = better
      - Ping (10%): lower = better
  return best scoring instance
```

### 6.5 Deployment Controller

**Responsibility:** Plugin and config rollouts.

**Module structure:**
```text
services/deployment/
├── cmd/main.go
├── internal/
│   ├── rollout/                # Deployment strategies
│   │   ├── rolling.go
│   │   ├── canary.go
│   │   └── all_at_once.go
│   ├── canary/                 # Canary analysis
│   │   └── compare.go          # Compare metrics before/after
│   └── config/                 # Config distribution
│       └── distribute.go
```

### 6.6 Backup Engine

**Responsibility:** Scheduled backups, consistent snapshots, disaster recovery.

**Module structure:**
```text
services/backup/
├── cmd/main.go
├── internal/
│   ├── scheduler/              # Backup scheduling
│   ├── snapshot/               # Consistent snapshot coordination
│   │   ├── prepare.go          # NexaLink: SAVE → FLUSH → SNAPSHOT_READY
│   │   └── resume.go           # Resume after snapshot
│   ├── storage/                # Storage backends
│   │   ├── local.go
│   │   ├── s3.go
│   │   ├── r2.go
│   │   ├── b2.go
│   │   └── sftp.go
│   └── restore/                # Restore logic
│       └── restore.go
```

### 6.7 Monitoring (Health Engine)

**Responsibility:** Health checks, Nexa Pulse, smart alerting.

**Module structure:**
```text
services/monitoring/
├── cmd/main.go
├── internal/
│   ├── health/                 # Health checks
│   │   ├── heartbeat.go
│   │   ├── tps_check.go
│   │   └── process_check.go
│   ├── pulse/                  # Nexa Pulse status computation
│   │   └── compute.go
│   ├── alerting/               # Smart alerting
│   │   ├── trend.go
│   │   └── correlate.go        # Link alerts to timeline
│   └── metrics/                # Metrics collection + aggregation
│       └── collect.go
```

### 6.8 NexaAgent

**Responsibility:** Daemon running on each VPS. Manages Docker containers, collects telemetry, executes commands.

**Module structure:**
```text
agent/
├── cmd/
│   └── main.go                 # Entry point
├── internal/
│   ├── docker/                 # Docker container management
│   │   ├── lifecycle.go        # Start/stop/create/delete containers
│   │   ├── stats.go            # Container stats (CPU, RAM, disk, network)
│   │   └── health.go           # Docker health checks
│   ├── telemetry/              # System + Minecraft telemetry
│   │   ├── system.go           # CPU, RAM, disk, network, temp, load
│   │   ├── minecraft.go        # Tunnel NexaLink data to control plane
│   │   └── heartbeat.go
│   ├── tunnel/                 # Tunnel Minecraft ports for NexaLink
│   ├── command/                # Command handler
│   │   ├── start.go
│   │   ├── stop.go
│   │   ├── create.go
│   │   ├── delete.go
│   │   ├── snapshot.go
│   │   └── migrate.go
│   ├── identity/               # mTLS identity management
│   └── config/                 # Agent config
```

### 6.9 NexaLink (Minecraft Plugins)

Three plugins: Paper, Folia, Velocity.

**Shared module structure:**
```text
minecraft/nexalink-{paper|folia|velocity}/
├── src/main/java/
│   └── dev/nexacloud/nexalink/
│       ├── NexaLinkPlugin.java       # Main plugin class
│       ├── telemetry/
│       │   ├── TelemetryCollector.java  # TPS, MSPT, players, chunks, etc.
│       │   ├── PlayerTracker.java
│       │   └── WorldTracker.java
│       ├── lifecycle/
│       │   ├── ShutdownHandler.java    # prepareShutdown, drainPlayers
│       │   ├── SaveHandler.java        # saveWorlds, flush
│       │   └── SnapshotHandler.java    # snapshotReady
│       ├── health/
│       │   └── HealthChecker.java    # healthCheck
│       ├── proxy/
│       │   └── VelocityProxy.java      # (Velocity only: registerServer, unregister)
│       └── protocol/
│           └── TelemetryProtocol.java  # Send data to agent
└── pom.xml                           # Maven build
```

**Tel protocol (NexaLink → Agent → Control Plane):**
- Uses a persistent TCP connection tunneled through the agent
- Binary protocol with protobuf or custom JSON over WebSocket
- Metrics sent every 5s
- Command responses (healthcheck results, etc.) sent on demand

### 6.10 CLI

```text
cli/
├── cmd/
│   ├── root.go
│   ├── status.go           # nexa status
│   ├── service.go          # nexa service scale <name> <count>
│   ├── deploy.go           # nexa deploy <file> --service <name>
│   ├── node.go             # nexa node drain <name>
│   ├── backup.go           # nexa network backup
│   └── instance.go         # nexa instance restart <name>
└── internal/
    ├── api/                # API client
    ├── config/             # Config management
    └── output/             # Output formatting
```

### 6.11 Dashboard

```text
apps/dashboard/
├── src/
│   ├── pages/              # Overview, Network, Services, Nodes, etc.
│   ├── components/         # NetworkMap, InstanceCard, Timeline, etc.
│   ├── hooks/              # useWebSocket, useApi
│   └── lib/                # API client, types
├── package.json
└── vite.config.ts
```

---

## 7. API Design

### Public REST API (API-first)

```yaml
GET  /api/v1/nodes                      # List all nodes
GET  /api/v1/nodes/{id}                 # Get node details
POST /api/v1/nodes/{id}/drain           # Drain a node

GET  /api/v1/services                   # List services
GET  /api/v1/services/{name}/instances   # List instances for service
POST /api/v1/services/{name}/scale      # Scale a service

GET  /api/v1/instances                  # List all instances
GET  /api/v1/instances/{name}            # Get instance details
POST /api/v1/instances/{name}/restart    # Restart an instance
POST /api/v1/instances/{name}/stop       # Stop an instance

GET  /api/v1/players                    # List online players
GET  /api/v1/players/{name}             # Player details
POST /api/v1/players/{name}/send       # Send player to server

GET  /api/v1/deployments                # List deployments
POST /api/v1/deployments                # Create deployment

GET  /api/v1/backups                    # List backups
POST /api/v1/backups                    # Create backup

GET  /api/v1/timeline                   # Timeline events

GET  /api/v1/metrics                    # Aggregated metrics
GET  /api/v1/metrics/players            # Player graph data
GET  /api/v1/metrics/tps                # TPS graph data
```

### WebSocket API (real-time)
```
Endpoint: ws://api/v1/ws

Events:
  instance.status     # Instance status changed
  node.status          # Node status changed
  player.join          # Player joined
  player.quit          # Player quit
  timeline.event       # New timeline event
  metrics.update       # Updated aggregated metrics
  deployment.progress  # Deployment progress update
```

### gRPC (internal service communication)
- `scheduler.Scheduler` — assign node, allocate ports
- `registry.Registry` — register/unregister, route queries
- `orchestrator.Orchestrator` — lifecycle commands
- `deployment.Deployment` — rollout management

### NATS Subjects
```text
Commands (request/reply):
  commands.node.{nodeID}.start_instance
  commands.node.{nodeID}.stop_instance
  commands.node.{nodeID}.create_instance
  commands.node.{nodeID}.delete_instance
  commands.node.{nodeID}.execute

Telemetry (pub/sub streams):
  telemetry.heartbeat.{nodeID}
  telemetry.metrics.{nodeID}
  telemetry.minecraft.{instanceID}
  telemetry.instance.status.{instanceID}

Events:
  events.instance.{action}     # created, started, stopped, crashed
  events.node.{action}         # joined, left, drained
  events.deployment.{action}
  events.backup.{action}
  events.alert.{severity}
```

---

## 8. Instance Lifecycle

States and transitions:
```text
           +------------+
           |  CREATING  |
           +-----+------+
                 |
           +-----v------+
           |  STARTING  |
           +-----+------+
                 |
           +-----v------+
           |   READY    |
           +-----+------+
                 |
           +-----v------+
           |   ACTIVE   |
           +-----+------+
          /        |  \
         v         v   v
  +-----------+  +-----------+  +-------------+
  |  DRAINING |  |  STOPPED   |  | CRASHED     |
  +-----+-----+  +-----+------+  +------+------+
        |              |                |
        v              v                |
  +-----------+  +-----------+        |
  |   READY   |  |           |        |
  +-----------+  |  (recreate)|        |
                 +-----------+        |
                                      v
                              +-------------+
                              | QUARANTINED |
                              +-------------+
```

Nexa Shift (blue/green replacement):
```text
Current: SkyBlock-02 (v1.21.10)
  1. Create SkyBlock-02-next (v1.21.11, new plugins/configs)
  2. Start → Healthcheck → Ready
  3. SkyBlock-02 → DRAINING
  4. Evacuate players
  5. When players=0: delete SkyBlock-02
  6. Rename SkyBlock-02-next → SkyBlock-02
```

---

## 9. Implementation Roadmap

### Phase 1: Foundation (MVP - basic orchestration)
- [ ] Project structure, Go workspace, linting
- [ ] PostgreSQL schema + migrations
- [ ] NATS setup + core messaging
- [ ] Core data models (instances, nodes, services)
- [ ] Orchestrator: basic reconciliation (create/stop instances)
- [ ] Scheduler: basic node selection (resource-based)
- [ ] Registry: register/unregister + proxy sync
- [ ] Agent: Docker lifecycle + telemetry (CPU/RAM/disk)
- [ ] API gateway: basic REST endpoints
- [ ] CLI: status, scale
- [ ] Docker Compose for dev environment

**Deliverable:** Basic network lifecycle management — define services, auto-create/terminate instances, basic health checks.

### Phase 2: Minecraft-Awareness
- [ ] NexaLink plugins (Paper + Velocity, Folia later)
- [ ] Telemetry integration (TPS, MSPT, players)
- [ ] Service discovery (dynamic Velocity registration)
- [ ] Smart routing (TPS/MSPT/player-based)
- [ ] Gracefult shutdown / drain
- [ ] Instance lifecycle states (DRAINING, etc.)

**Deliverable:** Minecraft-aware orchestration with real-time telemetry.

### Phase 3: Scaling & Self-Healing
- [ ] Auto-scaling (player, CPU, MSPT thresholds)
- [ ] Warm pools
- [ ] Wake-on-join
- [ ] Self-healing (restart policies)
- [ ] Crash loop detection + quarantine
- [ ] Dependency engine (ordered startup)
- [ ] Nexa Pulse health scores

**Deliverable:** Autonomous, self-healing network management.

### Phase 4: Advanced Operations
- [ ] Nexa Shift (blue/green replacement)
- [ ] Rolling restart
- [ ] Canary deployments
- [ ] Player evacuation
- [ ] Backup engine + consistent snapshots
- [ ] Disaster recovery
- [ ] Maintenance orchestrator

**Deliverable:** Zero-downtime deployments, backups, DR.

### Phase 5: UX Excellence
- [ ] Full React dashboard (network map, timeline, observability)
- [ ] Command palette
- [ ] Global console
- [ ] Nexa Timeline
- [ ] Smart alerting
- [ ] Automation engine + workflows
- [ ] Multi-region support

**Deliverable:** Production-ready platform with excellent UX.

### Phase 6: Enterprise Features
- [ ] RBAC + permissions
- [ ] Audit log
- [ ] Secrets manager (encryption at rest)
- [ ] Firewall automation
- [ ] Versioned configs
- [ ] Autopilot modes

**Deliverable:** Enterprise-grade secure platform.

---

## 10. Development Workflow

### Building Go services:
```bash
# From repo root
go build ./services/...
go test ./services/...
golangci-lint run
```

### Building the agent:
```bash
go build -o bin/nexa-agent ./agent/
```

### Building the CLI:
```bash
go build -o bin/nexa ./cli/
```

### Building Minecraft plugins:
```bash
cd minecraft/nexalink-paper && mvn package
cd minecraft/nexalink-velocity && mvn package
```

### Frontend development:
```bash
cd apps/dashboard
npm install
npm run dev
npm run lint
npm run typecheck
```

### Docker Compose (dev):
```bash
docker compose -f docker/compose/docker-compose.yml up -d
```

---

## 11. Key Design Decisions

1. **No Kubernetes** — Keep accessible for Minecraft operators. Docker + custom orchestration is sufficient.
2. **Go for control plane** — Performance, concurrency, single binary, excellent for networking.
3. **NATS over Kafka/RabbitMQ** — Lightweight, designed for this use case, simpler ops.
4. **PostgreSQL over MySQL/MariaDB** — Better for complex queries, JSON support, reliability.
5. **Monorepo** — Simplifies cross-component changes and shared model definitions.
6. **gRPC internal + REST external** — Best of both worlds: performance internally, accessibility externally.
7. **Declarative API** — Users describe desired state; system reconciles. Not imperative commands.

---

## 12. Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| NATS complexity | Start with NATS streaming basics, add JetStream as needed |
| Docker on agent | Use Docker SDK, test across Docker versions |
| Minecraft protocol | Use existing libraries (Velocity API, Paper API) via NexaLink |
| Player evacuation correctness | Extensive simulation testing, gradual rollout |
| Cross-service consistency | Distributed transactions unlikely; use eventual consistency with reconciliation |

---

## 13. Testing Strategy

- **Go unit tests** — per-package with table-driven tests
- **Integration tests** — Docker Compose environment with mock NATS/Postgres
- **Go test** — mock agent responses, test orchestrator reconciliation
- **Simulation tests** — simulate crash loops, node failures, player load
- **Frontend tests** — Jest + React Testing Library for components
