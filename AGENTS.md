# AGENTS.md

## Build & Test

```bash
make              # Build all Go services
make test         # Run all Go tests
make lint         # Run golangci-lint
make docker-up    # Start dev stack (postgres, redis, nats)
make docker-down  # Stop dev stack
```

## Go Workspace

This is a Go workspace. All Go modules live under:
- `pkg/model` — shared types
- `services/{orchestrator,scheduler,registry,autoscaler,deployment,backup,monitoring}` — control plane services
- `agent` — NexaAgent (VPS daemon)
- `cli` — nexa CLI tool

## Coding Standards

- Go: use `golangci-lint run` to check. No comments unless explicitly needed.
- All services use: GORM for PostgreSQL, NATS for messaging.
- Shared types live in `pkg/model`. Do not duplicate types across services.
- Use `model.NewID()` for UUIDs, `model.Now()` for timestamps.
