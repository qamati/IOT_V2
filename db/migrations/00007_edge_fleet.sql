-- +goose Up
-- ============================================================================
-- Service 22 & 23: Edge Fleet, Device Assignments & Sync Journal
-- ============================================================================

CREATE TABLE IF NOT EXISTS edge (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    edge_key TEXT NOT NULL UNIQUE,
    edge_secret TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'OFFLINE', -- ONLINE | OFFLINE | SYNCING
    firmware_version TEXT NOT NULL DEFAULT 'v1.0.0-edge',
    hardware_model TEXT NOT NULL DEFAULT 'Advantech UNO-2271G',
    last_sync_ts BIGINT NOT NULL DEFAULT 0,
    backlog_count BIGINT NOT NULL DEFAULT 0,
    assigned_devices JSONB NOT NULL DEFAULT '[]',
    assigned_profiles JSONB NOT NULL DEFAULT '[]',
    downlink_seq BIGINT NOT NULL DEFAULT 0,
    uplink_seq BIGINT NOT NULL DEFAULT 0,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_edge_tenant_name UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_edge_tenant ON edge (tenant_id);
CREATE INDEX IF NOT EXISTS idx_edge_key ON edge (edge_key);

CREATE TABLE IF NOT EXISTS edge_device_assignment (
    edge_id UUID NOT NULL REFERENCES edge(id) ON DELETE CASCADE,
    device_id UUID NOT NULL REFERENCES device(id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (edge_id, device_id)
);

CREATE INDEX IF NOT EXISTS idx_edge_dev_assign_dev ON edge_device_assignment (device_id);

CREATE TABLE IF NOT EXISTS edge_sync_journal (
    id UUID PRIMARY KEY,
    edge_id UUID NOT NULL REFERENCES edge(id) ON DELETE CASCADE,
    sequence BIGINT NOT NULL,
    lane_l0_count INT NOT NULL DEFAULT 0,
    lane_l1_count INT NOT NULL DEFAULT 0,
    lane_l2_count INT NOT NULL DEFAULT 0,
    processed_items INT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'PROCESSED',
    synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_edge_sync_seq UNIQUE (edge_id, sequence)
);

CREATE INDEX IF NOT EXISTS idx_edge_sync_edge ON edge_sync_journal (edge_id, sequence DESC);

-- Row-Level Security
ALTER TABLE edge ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS edge_tenant_isolation ON edge;
CREATE POLICY edge_tenant_isolation ON edge FOR ALL USING (
    current_setting('app.current_tenant_id', true) IS NULL OR
    current_setting('app.current_tenant_id', true) = '' OR
    tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::UUID OR
    tenant_id IS NULL
);

ALTER TABLE edge_device_assignment ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS edge_assignment_isolation ON edge_device_assignment;
CREATE POLICY edge_assignment_isolation ON edge_device_assignment FOR ALL USING (
    EXISTS (SELECT 1 FROM edge e WHERE e.id = edge_device_assignment.edge_id)
);

ALTER TABLE edge_sync_journal ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS edge_sync_journal_isolation ON edge_sync_journal;
CREATE POLICY edge_sync_journal_isolation ON edge_sync_journal FOR ALL USING (
    EXISTS (SELECT 1 FROM edge e WHERE e.id = edge_sync_journal.edge_id)
);

-- +goose Down
DROP TABLE IF EXISTS edge_sync_journal CASCADE;
DROP TABLE IF EXISTS edge_device_assignment CASCADE;
DROP TABLE IF EXISTS edge CASCADE;
