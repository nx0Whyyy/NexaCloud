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
    organization_id UUID,
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
-- User identities and browser sessions
-- ------------------------------------------------------------------
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username        TEXT UNIQUE NOT NULL,
    email           TEXT UNIQUE NOT NULL,
    password_hash   TEXT NOT NULL,
    role            TEXT NOT NULL DEFAULT 'user',
    email_verified_at TIMESTAMPTZ,
    disabled_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ------------------------------------------------------------------
-- Tenancy, subscriptions and server-side entitlements
-- ------------------------------------------------------------------
CREATE TABLE organizations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    slug        TEXT UNIQUE NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE organization_members (
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role            TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (organization_id, user_id)
);

CREATE TABLE plans (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code                TEXT UNIQUE NOT NULL,
    name                TEXT NOT NULL,
    price_monthly       BIGINT NOT NULL DEFAULT 0,
    active              BOOLEAN NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE plan_entitlements (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id     UUID NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    key         TEXT NOT NULL,
    value       TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (plan_id, key)
);

CREATE TABLE subscriptions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     UUID UNIQUE NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    plan_id             UUID NOT NULL REFERENCES plans(id),
    status              TEXT NOT NULL,
    entitlement_data    JSONB NOT NULL,
    current_period_ends TIMESTAMPTZ,
    grace_ends_at       TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE licenses (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key_prefix          TEXT NOT NULL,
    key_hash            TEXT UNIQUE NOT NULL,
    status              TEXT NOT NULL DEFAULT 'ACTIVE',
    last_used_at        TIMESTAMPTZ,
    expires_at          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at          TIMESTAMPTZ
);

CREATE TABLE networks (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name                TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'ACTIVE',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, name)
);

CREATE TABLE instance_slots (
    instance_id         UUID PRIMARY KEY REFERENCES instances(id) ON DELETE CASCADE,
    organization_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    reserved_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE node_credentials (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id     UUID NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    public_key  TEXT NOT NULL,
    fingerprint TEXT UNIQUE NOT NULL,
    status      TEXT NOT NULL DEFAULT 'ACTIVE',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ,
    revoked_at  TIMESTAMPTZ
);

CREATE TABLE device_enrollments (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code_hash           TEXT UNIQUE NOT NULL,
    device_token_hash   TEXT UNIQUE NOT NULL,
    node_name           TEXT NOT NULL,
    public_key          TEXT NOT NULL,
    resources           JSONB NOT NULL DEFAULT '{}',
    status              TEXT NOT NULL DEFAULT 'PENDING',
    node_id             UUID REFERENCES nodes(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at          TIMESTAMPTZ NOT NULL,
    approved_at         TIMESTAMPTZ,
    claimed_at          TIMESTAMPTZ
);

ALTER TABLE nodes ADD COLUMN organization_id UUID REFERENCES organizations(id);
ALTER TABLE nodes ADD COLUMN network_id UUID REFERENCES networks(id);
ALTER TABLE services ADD COLUMN organization_id UUID REFERENCES organizations(id);
ALTER TABLE services ADD COLUMN network_id UUID REFERENCES networks(id);
ALTER TABLE instances ADD COLUMN organization_id UUID REFERENCES organizations(id);
ALTER TABLE instances ADD COLUMN network_id UUID REFERENCES networks(id);
ALTER TABLE audit_log ADD CONSTRAINT fk_audit_organization FOREIGN KEY (organization_id) REFERENCES organizations(id);

CREATE TABLE user_sessions (
    token_hash      TEXT PRIMARY KEY,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE email_verifications (
    token_hash  TEXT PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ------------------------------------------------------------------
-- Public status history and incidents
-- ------------------------------------------------------------------
CREATE TABLE snapshots (
    id              BIGSERIAL PRIMARY KEY,
    component       TEXT NOT NULL,
    status          TEXT NOT NULL,
    latency_ms      BIGINT NOT NULL DEFAULT 0,
    checked_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE incidents (
    id              BIGSERIAL PRIMARY KEY,
    component       TEXT NOT NULL,
    title           TEXT NOT NULL,
    status          TEXT NOT NULL,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at     TIMESTAMPTZ,
    last_message    TEXT
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
CREATE INDEX idx_user_sessions_user ON user_sessions(user_id);
CREATE INDEX idx_user_sessions_expiry ON user_sessions(expires_at);
CREATE INDEX idx_email_verifications_user ON email_verifications(user_id);
CREATE INDEX idx_email_verifications_expiry ON email_verifications(expires_at);
CREATE INDEX idx_organization_members_user ON organization_members(user_id);
CREATE INDEX idx_licenses_organization ON licenses(organization_id);
CREATE INDEX idx_networks_organization ON networks(organization_id);
CREATE INDEX idx_instance_slots_organization ON instance_slots(organization_id);
CREATE INDEX idx_node_credentials_node ON node_credentials(node_id);
CREATE INDEX idx_device_enrollments_expiry ON device_enrollments(expires_at);
CREATE INDEX idx_snapshots_component ON snapshots(component);
CREATE INDEX idx_snapshots_checked_at ON snapshots(checked_at DESC);
CREATE INDEX idx_incidents_component ON incidents(component);
CREATE INDEX idx_incidents_status ON incidents(status);
