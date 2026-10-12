-- +goose Up
-- Migration 00002: Extended Entities, Relations, Attributes, Alarms, Dashboards, RPC, and SCADA Tags

-- 1. Customer hierarchy
CREATE TABLE IF NOT EXISTS customer (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  tenant_id uuid NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  parent_customer_id uuid REFERENCES customer(id),
  title text NOT NULL,
  email text,
  phone text,
  country text,
  city text,
  address text,
  is_public boolean NOT NULL DEFAULT false,
  additional_info jsonb NOT NULL DEFAULT '{}'::jsonb,
  version integer NOT NULL DEFAULT 1,
  UNIQUE (tenant_id, title)
);
CREATE INDEX IF NOT EXISTS customer_parent_idx ON customer (tenant_id, parent_customer_id);

-- 2. Asset Profile & Asset entities
CREATE TABLE IF NOT EXISTS asset_profile (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  tenant_id uuid NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  name text NOT NULL,
  description text,
  image text,
  is_default boolean NOT NULL DEFAULT false,
  default_rule_chain_id uuid,
  default_dashboard_id uuid,
  default_queue_name text NOT NULL DEFAULT 'main',
  profile_data jsonb NOT NULL DEFAULT '{}'::jsonb,
  schema_version smallint NOT NULL DEFAULT 1,
  version integer NOT NULL DEFAULT 1,
  UNIQUE (tenant_id, name)
);

CREATE TABLE IF NOT EXISTS asset (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  tenant_id uuid NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  customer_id uuid REFERENCES customer(id) ON DELETE SET NULL,
  asset_profile_id uuid NOT NULL REFERENCES asset_profile(id),
  name text NOT NULL,
  label text,
  additional_info jsonb NOT NULL DEFAULT '{}'::jsonb,
  tags text[] NOT NULL DEFAULT '{}',
  version integer NOT NULL DEFAULT 1,
  UNIQUE (tenant_id, name)
);
CREATE INDEX IF NOT EXISTS asset_tenant_idx ON asset (tenant_id);
CREATE INDEX IF NOT EXISTS asset_customer_idx ON asset (tenant_id, customer_id);
CREATE INDEX IF NOT EXISTS asset_profile_idx ON asset (tenant_id, asset_profile_id);

-- 3. General Entity Relations Graph
CREATE TABLE IF NOT EXISTS relation (
  tenant_id uuid NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  from_id uuid NOT NULL,
  from_type text NOT NULL,
  to_id uuid NOT NULL,
  to_type text NOT NULL,
  relation_type_group text NOT NULL DEFAULT 'COMMON',
  relation_type text NOT NULL,
  additional_info jsonb NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY (from_id, relation_type_group, relation_type, to_id)
);
CREATE INDEX IF NOT EXISTS relation_to_idx ON relation (to_id, relation_type_group, relation_type, from_id);
CREATE INDEX IF NOT EXISTS relation_tenant_idx ON relation (tenant_id);

-- 4. Scoped Attributes Table (CLIENT_SCOPE=1, SERVER_SCOPE=2, SHARED_SCOPE=3)
CREATE TABLE IF NOT EXISTS attribute_kv (
  entity_id uuid NOT NULL,
  scope smallint NOT NULL,
  key_id integer NOT NULL REFERENCES key_dictionary(key_id) ON DELETE CASCADE,
  bool_v boolean,
  str_v text,
  long_v bigint,
  dbl_v double precision,
  json_v jsonb,
  last_update_ts bigint NOT NULL,
  version bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (entity_id, scope, key_id)
);
CREATE INDEX IF NOT EXISTS attribute_kv_entity_scope_idx ON attribute_kv (entity_id, scope);

-- 5. Alarms & Alarm Comments
CREATE TABLE IF NOT EXISTS alarm (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  tenant_id uuid NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  customer_id uuid REFERENCES customer(id) ON DELETE SET NULL,
  type text NOT NULL,
  originator_id uuid NOT NULL,
  originator_type text NOT NULL DEFAULT 'DEVICE',
  severity text NOT NULL CHECK (severity IN ('CRITICAL','MAJOR','MINOR','WARNING','INDETERMINATE')),
  status text NOT NULL CHECK (status IN ('ACTIVE_UNACK','ACTIVE_ACK','CLEARED_UNACK','CLEARED_ACK')),
  acknowledged boolean NOT NULL DEFAULT false,
  cleared boolean NOT NULL DEFAULT false,
  assignee_id uuid,
  start_ts bigint NOT NULL,
  end_ts bigint,
  ack_ts bigint,
  clear_ts bigint,
  details jsonb NOT NULL DEFAULT '{}'::jsonb,
  propagate boolean NOT NULL DEFAULT false,
  isa_state text,
  version integer NOT NULL DEFAULT 1
);
CREATE UNIQUE INDEX IF NOT EXISTS alarm_active_uq ON alarm (originator_id, type) WHERE cleared = false;
CREATE INDEX IF NOT EXISTS alarm_tenant_time_idx ON alarm (tenant_id, created_time DESC);

CREATE TABLE IF NOT EXISTS alarm_comment (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  alarm_id uuid NOT NULL REFERENCES alarm(id) ON DELETE CASCADE,
  user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  type text NOT NULL,
  comment jsonb NOT NULL
);

-- 6. Dashboards
CREATE TABLE IF NOT EXISTS dashboard (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  tenant_id uuid NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  title text NOT NULL,
  image text,
  assigned_customers jsonb NOT NULL DEFAULT '[]'::jsonb,
  configuration jsonb NOT NULL DEFAULT '{}'::jsonb,
  version integer NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS dashboard_tenant_idx ON dashboard (tenant_id);

-- 7. Downlink RPC History
CREATE TABLE IF NOT EXISTS rpc (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  tenant_id uuid NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  device_id uuid NOT NULL REFERENCES device(id) ON DELETE CASCADE,
  method text NOT NULL,
  params jsonb,
  response jsonb,
  status text NOT NULL,
  two_way boolean NOT NULL DEFAULT false,
  expiration_time bigint NOT NULL,
  additional_info jsonb NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX IF NOT EXISTS rpc_device_idx ON rpc (tenant_id, device_id, created_time DESC);

-- 8. Industrial SCADA Process Tags
CREATE TABLE IF NOT EXISTS scada_tag (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  tenant_id uuid NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  device_id uuid REFERENCES device(id) ON DELETE SET NULL,
  path text NOT NULL,
  data_type text NOT NULL,
  eng_unit text,
  deadband double precision NOT NULL DEFAULT 0.0,
  scale_factor double precision NOT NULL DEFAULT 1.0,
  scale_offset double precision NOT NULL DEFAULT 0.0,
  eu_min double precision NOT NULL DEFAULT 0.0,
  eu_max double precision NOT NULL DEFAULT 100.0,
  scan_rate_ms integer NOT NULL DEFAULT 1000,
  val_json jsonb,
  quality text NOT NULL DEFAULT 'GOOD',
  timestamp bigint NOT NULL,
  version integer NOT NULL DEFAULT 1,
  UNIQUE (tenant_id, path)
);
CREATE INDEX IF NOT EXISTS scada_tag_tenant_path_idx ON scada_tag (tenant_id, path);

-- Multi-tenancy RLS Policies
ALTER TABLE customer ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS customer_tenant_isolation ON customer;
CREATE POLICY customer_tenant_isolation ON customer FOR ALL USING (
  current_setting('app.tenant_id', true) IS NULL OR
  current_setting('app.tenant_id', true) = '' OR
  tenant_id::text = current_setting('app.tenant_id', true)
);

ALTER TABLE asset_profile ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS asset_profile_tenant_isolation ON asset_profile;
CREATE POLICY asset_profile_tenant_isolation ON asset_profile FOR ALL USING (
  current_setting('app.tenant_id', true) IS NULL OR
  current_setting('app.tenant_id', true) = '' OR
  tenant_id::text = current_setting('app.tenant_id', true)
);

ALTER TABLE asset ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS asset_tenant_isolation ON asset;
CREATE POLICY asset_tenant_isolation ON asset FOR ALL USING (
  current_setting('app.tenant_id', true) IS NULL OR
  current_setting('app.tenant_id', true) = '' OR
  tenant_id::text = current_setting('app.tenant_id', true)
);

ALTER TABLE relation ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS relation_tenant_isolation ON relation;
CREATE POLICY relation_tenant_isolation ON relation FOR ALL USING (
  current_setting('app.tenant_id', true) IS NULL OR
  current_setting('app.tenant_id', true) = '' OR
  tenant_id::text = current_setting('app.tenant_id', true)
);

ALTER TABLE alarm ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS alarm_tenant_isolation ON alarm;
CREATE POLICY alarm_tenant_isolation ON alarm FOR ALL USING (
  current_setting('app.tenant_id', true) IS NULL OR
  current_setting('app.tenant_id', true) = '' OR
  tenant_id::text = current_setting('app.tenant_id', true)
);

ALTER TABLE dashboard ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS dashboard_tenant_isolation ON dashboard;
CREATE POLICY dashboard_tenant_isolation ON dashboard FOR ALL USING (
  current_setting('app.tenant_id', true) IS NULL OR
  current_setting('app.tenant_id', true) = '' OR
  tenant_id::text = current_setting('app.tenant_id', true)
);

ALTER TABLE rpc ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS rpc_tenant_isolation ON rpc;
CREATE POLICY rpc_tenant_isolation ON rpc FOR ALL USING (
  current_setting('app.tenant_id', true) IS NULL OR
  current_setting('app.tenant_id', true) = '' OR
  tenant_id::text = current_setting('app.tenant_id', true)
);

ALTER TABLE scada_tag ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS scada_tag_tenant_isolation ON scada_tag;
CREATE POLICY scada_tag_tenant_isolation ON scada_tag FOR ALL USING (
  current_setting('app.tenant_id', true) IS NULL OR
  current_setting('app.tenant_id', true) = '' OR
  tenant_id::text = current_setting('app.tenant_id', true)
);

-- +goose Down
DROP TABLE IF EXISTS scada_tag;
DROP TABLE IF EXISTS rpc;
DROP TABLE IF EXISTS dashboard;
DROP TABLE IF EXISTS alarm_comment;
DROP TABLE IF EXISTS alarm;
DROP TABLE IF EXISTS attribute_kv;
DROP TABLE IF EXISTS relation;
DROP TABLE IF EXISTS asset;
DROP TABLE IF EXISTS asset_profile;
DROP TABLE IF EXISTS customer;
