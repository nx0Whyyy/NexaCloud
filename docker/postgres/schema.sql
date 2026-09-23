-- ============================================================
-- NexaCloud Database Schema (PostgreSQL)
-- All tables use UUID primary keys (except configs/versions).
-- ============================================================

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ------------------------------------------------------------------
-- Nodes: physical/virtual machines running nexa-agent
-- ------------------------------------------------------------------
CREATE TABLE nodes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT UNIQUE NOT NULL,
    status          TEXT NOT NULL DEFAULT 'OFFLINE',
    labels          JSONB DEFAULT '{}',
    resources       JSONB NOT NULL, -- {memory, cpu, disk}
    usage           JSONB DEFAULT '{"cpu":0,"ram":0,"disk":0,"network":0}',
    agent_version   TEXT,
    last_heartbeat  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ------------------------------------------------------------------
-- Services: logical groups (e.g. "skyblock", "lobby")
-- ------------------------------------------------------------------
CREATE TABLE services (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT UNIQUE NOT NULL,
    type            TEXT NOT NULL DEFAULT 'minecraft',
    software        JSONB NOT NULL, -- {type, version, java:{version}}
    resources       JSONB NOT NULL, -- {memory, cpu}
    autoscaling     JSONB,          -- {minimum, maximum, scaleUp, scaleDown, playersPerInstance}
    placement       JSONB,          -- {region, requirements, avoid, spread, antSplit}
    dependencies    TEXT[],
    config_template TEXT,
    blueprint_id    UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ------------------------------------------------------------------
-- Instances: actual running Minecraft server containers
-- ------------------------------------------------------------------
CREATE TABLE instances (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT UNIQUE NOT NULL,
    service_name    TEXT NOT NULL,
    node_id         UUID REFERENCES nodes(id),
    status          TEXT NOT NULL DEFAULT 'CREATING',
    pulse           TEXT NOT NULL DEFAULT 'OFFLINE',
    container_id    TEXT,
    address         TEXT,
    port            INT,
    resources       JSONB NOT NULL,
    metadata        JSONB DEFAULT '{}',
    health          JSONB,
    player_count    INT NOT NULL DEFAULT 0,
    version         TEXT,
    plugins         TEXT[],
    config_version  TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at      TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ------------------------------------------------------------------
-- Registered services: instances known to the proxy
-- ------------------------------------------------------------------
CREATE TABLE registered_services (
    instance_id     UUID PRIMARY KEY REFERENCES instances(id),
    service_name    TEXT NOT NULL,
    address         TEXT NOT NULL,
    port            INT NOT NULL,
    ready           BOOLEAN NOT NULL DEFAULT false,
    registered_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ------------------------------------------------------------------
-- Version-controlled configurations
-- ------------------------------------------------------------------
CREATE TABLE configs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope           TEXT NOT NULL, -- global, service, instance
    path            TEXT NOT NULL, -- e.g. "global/messages.yml"
    content         TEXT,
    version         INT NOT NULL DEFAULT 1,
    created_by      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(scope, path, version)
);

-- ------------------------------------------------------------------
-- Backups
-- ------------------------------------------------------------------
CREATE TABLE backups (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id     UUID REFERENCES instances(id),
    type            TEXT NOT NULL, -- full, world, config, plugin, database
    destination     TEXT NOT NULL, -- local, s3, r2, b2, minio, ftp, sftp
    path            TEXT,
    size            BIGINT,
    status          TEXT NOT NULL DEFAULT 'pending',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ------------------------------------------------------------------
-- Deployments (plugin/config rollouts)
-- ------------------------------------------------------------------
CREATE TABLE deployments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id      UUID REFERENCES services(id),
    target_version  TEXT,
    plugin_name     TEXT,
    version         TEXT,
    strategy        TEXT NOT NULL DEFAULT 'rolling',
    canary_percentage INT,
    current_phase     TEXT,
    completed_instances TEXT[],
    failed_instances  TEXT[],
    status          TEXT NOT NULL DEFAULT 'pending',
    created_by      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);

-- ------------------------------------------------------------------
-- Timeline events
-- ------------------------------------------------------------------
CREATE TABLE timeline_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor           TEXT NOT NULL,
    action          TEXT NOT NULL,
    resource_type   TEXT NOT NULL,
    resource_id     TEXT NOT NULL,
    details         TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ------------------------------------------------------------------
-- Audit log
-- ------------------------------------------------------------------
CREATE TABLE audit_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor           TEXT NOT NULL,
    action          TEXT NOT NULL,
    resource_type   TEXT NOT NULL,
    resource_id     TEXT NOT NULL,
    ip_address      TEXT,
    result          TEXT NOT NULL,
    details         TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ------------------------------------------------------------------
-- Secrets (value_encrypted stored server-side)
-- ------------------------------------------------------------------
CREATE TABLE secrets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key             TEXT UNIQUE NOT NULL,
    value_encrypted TEXT NOT NULL,
    scope           TEXT NOT NULL DEFAULT 'global',
    rotated_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ------------------------------------------------------------------
-- Indexes
-- ------------------------------------------------------------------
CREATE INDEX idx_instances_service ON instances(service_name);
CREATE INDEX idx_instances_status  ON instances(status);
CREATE INDEX idx_instances_node    ON instances(node_id);
CREATE INDEX idx_instances_pulse   ON instances(pulse);
CREATE INDEX idx_timeline_time     ON timeline_events(created_at DESC);
CREATE INDEX idx_nodes_heartbeat   ON nodes(last_heartbeat DESC);
CREATE INDEX idx_audit_time        ON audit_log(created_at DESC);
CREATE INDEX idx_configs_scope     ON configs(scope, path);
