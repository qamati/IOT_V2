-- +goose Up
-- ============================================================================
-- Service 18 & 19: Integrations, Converters, and Field Gateway Checkpoints
-- ============================================================================

CREATE TABLE IF NOT EXISTS converter (
    id UUID PRIMARY KEY,
    tenant_id UUID REFERENCES tenant(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    direction TEXT NOT NULL DEFAULT 'UPLINK', -- UPLINK | DOWNLINK
    mode TEXT NOT NULL DEFAULT 'DECLARATIVE',  -- DECLARATIVE | EXPRESSION
    definition JSONB NOT NULL DEFAULT '{}',
    tests JSONB NOT NULL DEFAULT '[]',
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_converter_tenant_name ON converter (COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::UUID), name);

CREATE INDEX IF NOT EXISTS idx_converter_tenant ON converter (tenant_id);

CREATE TABLE IF NOT EXISTS integration (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    type TEXT NOT NULL, -- HTTP_WEBHOOK | MQTT_CLIENT | LORAWAN_CHIRPSTACK | TTN | OPC_UA | MODBUS_GATEWAY | KAFKA
    enabled BOOLEAN NOT NULL DEFAULT true,
    uplink_converter_id UUID REFERENCES converter(id) ON DELETE SET NULL,
    downlink_converter_id UUID REFERENCES converter(id) ON DELETE SET NULL,
    routing_key TEXT NOT NULL,
    config JSONB NOT NULL DEFAULT '{}',
    secrets_ref JSONB NOT NULL DEFAULT '{}',
    allow_create_device BOOLEAN NOT NULL DEFAULT true,
    default_device_type TEXT NOT NULL DEFAULT 'generic',
    remote BOOLEAN NOT NULL DEFAULT false,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_integration_tenant_name UNIQUE (tenant_id, name),
    CONSTRAINT uq_integration_routing_key UNIQUE (routing_key)
);

CREATE INDEX IF NOT EXISTS idx_integration_tenant ON integration (tenant_id);
CREATE INDEX IF NOT EXISTS idx_integration_routing ON integration (routing_key);

CREATE TABLE IF NOT EXISTS integration_status (
    integration_id UUID PRIMARY KEY REFERENCES integration(id) ON DELETE CASCADE,
    state TEXT NOT NULL DEFAULT 'RUNNING',
    last_error TEXT,
    last_ok_ts BIGINT NOT NULL DEFAULT 0,
    counters JSONB NOT NULL DEFAULT '{}',
    instance TEXT NOT NULL DEFAULT 'standalone'
);

CREATE TABLE IF NOT EXISTS integration_checkpoint (
    integration_id UUID REFERENCES integration(id) ON DELETE CASCADE,
    partition_key TEXT NOT NULL,
    position BYTEA NOT NULL,
    updated_ts BIGINT NOT NULL,
    PRIMARY KEY (integration_id, partition_key)
);

ALTER TABLE converter ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS converter_tenant_isolation ON converter;
CREATE POLICY converter_tenant_isolation ON converter FOR ALL USING (
    current_setting('app.current_tenant_id', true) IS NULL OR
    current_setting('app.current_tenant_id', true) = '' OR
    tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::UUID OR
    tenant_id IS NULL
);

ALTER TABLE integration ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS integration_tenant_isolation ON integration;
CREATE POLICY integration_tenant_isolation ON integration FOR ALL USING (
    current_setting('app.current_tenant_id', true) IS NULL OR
    current_setting('app.current_tenant_id', true) = '' OR
    tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::UUID OR
    tenant_id IS NULL
);

-- +goose Down
DROP TABLE IF EXISTS integration_checkpoint CASCADE;
DROP TABLE IF EXISTS integration_status CASCADE;
DROP TABLE IF EXISTS integration CASCADE;
DROP TABLE IF EXISTS converter CASCADE;
