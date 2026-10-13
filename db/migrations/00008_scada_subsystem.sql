-- +goose Up
-- ============================================================================
-- Service 24: Industrial SCADA Subsystem, ISA-18.2 Alarms & ThingsBoard Symbols
-- ============================================================================

-- 1. ISA-18.2 Process Alarms
CREATE TABLE IF NOT EXISTS scada_alarm (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
    tag_path TEXT NOT NULL,
    alarm_type TEXT NOT NULL, -- HIGH_HIGH | HIGH | LOW | LOW_LOW | DEV | TRIP
    setpoint DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    severity TEXT NOT NULL DEFAULT 'HIGH', -- CRITICAL | HIGH | MEDIUM | LOW
    state TEXT NOT NULL DEFAULT 'UNACK_ALARM', -- NORMAL | UNACK_ALARM | ACKED_ALARM | RTN_UNACK | SHELVED | SUPPRESSED | OUT_OF_SERVICE
    active_ts BIGINT NOT NULL,
    ack_ts BIGINT,
    ack_user TEXT,
    shelve_until BIGINT,
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_scada_alarm_tenant ON scada_alarm (tenant_id);
CREATE INDEX IF NOT EXISTS idx_scada_alarm_tag ON scada_alarm (tenant_id, tag_path);
CREATE INDEX IF NOT EXISTS idx_scada_alarm_state ON scada_alarm (state);

-- 2. Select-Before-Operate (SBO) Control Log & Audit
CREATE TABLE IF NOT EXISTS scada_control_log (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
    tag_path TEXT NOT NULL,
    desired_value JSONB NOT NULL,
    operator TEXT NOT NULL DEFAULT 'system',
    status TEXT NOT NULL DEFAULT 'SELECTED', -- SELECTED | OPERATED | CANCELLED | TIMED_OUT | INTERLOCK_DENIED
    reason TEXT,
    selected_at BIGINT NOT NULL,
    operated_at BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_scada_control_tenant ON scada_control_log (tenant_id);
CREATE INDEX IF NOT EXISTS idx_scada_control_tag ON scada_control_log (tag_path);

-- 3. ThingsBoard-Compatible SVG Symbols Catalog
CREATE TABLE IF NOT EXISTS scada_symbol (
    id UUID PRIMARY KEY,
    tenant_id UUID REFERENCES tenant(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    category TEXT NOT NULL, -- Pumps | Valves | Tanks | Pipes | Meters | Motors
    style TEXT NOT NULL DEFAULT 'standard', -- standard | isa-101
    svg_content TEXT NOT NULL,
    tags JSONB NOT NULL DEFAULT '[]',
    properties JSONB NOT NULL DEFAULT '[]',
    behaviors JSONB NOT NULL DEFAULT '[]',
    animations JSONB NOT NULL DEFAULT '[]',
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_scada_symbol_tenant_name ON scada_symbol (COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::UUID), name);
CREATE INDEX IF NOT EXISTS idx_scada_symbol_tenant ON scada_symbol (tenant_id);
CREATE INDEX IF NOT EXISTS idx_scada_symbol_category ON scada_symbol (category);

-- Row-Level Security
ALTER TABLE scada_alarm ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS scada_alarm_tenant_isolation ON scada_alarm;
CREATE POLICY scada_alarm_tenant_isolation ON scada_alarm FOR ALL USING (
    current_setting('app.current_tenant_id', true) IS NULL OR
    current_setting('app.current_tenant_id', true) = '' OR
    tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::UUID OR
    tenant_id IS NULL
);

ALTER TABLE scada_control_log ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS scada_control_tenant_isolation ON scada_control_log;
CREATE POLICY scada_control_tenant_isolation ON scada_control_log FOR ALL USING (
    current_setting('app.current_tenant_id', true) IS NULL OR
    current_setting('app.current_tenant_id', true) = '' OR
    tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::UUID OR
    tenant_id IS NULL
);

ALTER TABLE scada_symbol ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS scada_symbol_tenant_isolation ON scada_symbol;
CREATE POLICY scada_symbol_tenant_isolation ON scada_symbol FOR ALL USING (
    current_setting('app.current_tenant_id', true) IS NULL OR
    current_setting('app.current_tenant_id', true) = '' OR
    tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::UUID OR
    tenant_id IS NULL
);

-- +goose Down
DROP TABLE IF EXISTS scada_symbol CASCADE;
DROP TABLE IF EXISTS scada_control_log CASCADE;
DROP TABLE IF EXISTS scada_alarm CASCADE;
