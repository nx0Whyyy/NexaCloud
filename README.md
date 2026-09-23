# NexaCloud

NexaCloud is a Minecraft-aware orchestration platform built around a Go control
plane and lightweight agents running on game-server nodes.

## Components

- `services/orchestrator`: instance lifecycle and reconciliation
- `services/scheduler`: node selection and workload placement
- `services/registry`: service discovery and routing
- `services/autoscaler`: capacity management
- `services/deployment`: deployment workflows
- `services/backup`: backup workflows
- `services/monitoring`: platform monitoring
- `agent`: node daemon and Docker workload management
- `cli`: command-line client
- `pkg/model`: shared domain types

## Development

Requirements: Go 1.23+, Docker, Docker Compose, and Make.

```bash
make docker-up
make test
make build
```

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the system design.
