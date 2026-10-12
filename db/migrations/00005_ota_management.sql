-- +goose Up
-- Migration 00005: OTA Packages, Firmware Distribution, and Device State

-- 1. OTA Packages (Firmware & Software binaries)
CREATE TABLE IF NOT EXISTS ota_package (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  tenant_id uuid NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  type text NOT NULL, -- FIRMWARE, SOFTWARE
  title text NOT NULL,
  version text NOT NULL,
  tag text,
  url text,
  file_name text,
  content_type text,
  checksum_algorithm text NOT NULL DEFAULT 'SHA256',
  checksum text NOT NULL,
  data_size bigint NOT NULL DEFAULT 0,
  data bytea,
  additional_info jsonb NOT NULL DEFAULT '{}'::jsonb,
  CONSTRAINT ota_pkg_unique_ver UNIQUE (tenant_id, type, title, version)
);
CREATE INDEX IF NOT EXISTS ota_pkg_tenant_idx ON ota_package (tenant_id, type);

-- 2. OTA Assignments (Profile or Device level)
CREATE TABLE IF NOT EXISTS ota_assignment (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  tenant_id uuid NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  kind text NOT NULL, -- FIRMWARE, SOFTWARE
  package_id uuid NOT NULL REFERENCES ota_package(id) ON DELETE CASCADE,
  scope text NOT NULL, -- DEVICE_PROFILE, DEVICE
  target_id uuid NOT NULL,
  CONSTRAINT ota_assign_unique UNIQUE (scope, target_id, kind)
);
CREATE INDEX IF NOT EXISTS ota_assign_target_idx ON ota_assignment (tenant_id, scope, target_id);

-- 3. Per-Device OTA Execution State
CREATE TABLE IF NOT EXISTS device_ota_state (
  device_id uuid NOT NULL REFERENCES device(id) ON DELETE CASCADE,
  kind text NOT NULL, -- FIRMWARE, SOFTWARE
  package_id uuid REFERENCES ota_package(id) ON DELETE SET NULL,
  state text NOT NULL, -- QUEUED, INITIATED, DOWNLOADING, DOWNLOADED, VERIFIED, UPDATING, UPDATED, FAILED, STALLED
  progress real NOT NULL DEFAULT 0.0,
  error text,
  updated_ts bigint NOT NULL,
  attempts int NOT NULL DEFAULT 0,
  PRIMARY KEY (device_id, kind)
);
CREATE INDEX IF NOT EXISTS device_ota_state_pkg_idx ON device_ota_state (package_id, state);

-- 4. Extend RPC Table for persistent retries and response timing
ALTER TABLE rpc ADD COLUMN IF NOT EXISTS retries int NOT NULL DEFAULT 0;
ALTER TABLE rpc ADD COLUMN IF NOT EXISTS max_retries int NOT NULL DEFAULT 3;
ALTER TABLE rpc ADD COLUMN IF NOT EXISTS response_time bigint;
ALTER TABLE rpc ADD COLUMN IF NOT EXISTS request_id text;

-- 5. Row-Level Security Policies for Multi-Tenancy
ALTER TABLE ota_package ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS ota_pkg_tenant_isolation ON ota_package;
CREATE POLICY ota_pkg_tenant_isolation ON ota_package FOR ALL USING (
  tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
);

ALTER TABLE ota_assignment ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS ota_assign_tenant_isolation ON ota_assignment;
CREATE POLICY ota_assign_tenant_isolation ON ota_assignment FOR ALL USING (
  tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
);

-- +goose Down
DROP TABLE IF EXISTS device_ota_state;
DROP TABLE IF EXISTS ota_assignment;
DROP TABLE IF EXISTS ota_package;
