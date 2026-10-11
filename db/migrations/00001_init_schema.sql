-- +goose Up
-- Core schema for iotp IoT platform

CREATE TABLE IF NOT EXISTS tenant_profile (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  name text NOT NULL,
  description text,
  is_default boolean NOT NULL DEFAULT false,
  isolation_level smallint NOT NULL DEFAULT 1,
  profile_data jsonb NOT NULL DEFAULT '{}'::jsonb,
  schema_version smallint NOT NULL DEFAULT 1,
  version integer NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS tenant (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  tenant_profile_id uuid NOT NULL REFERENCES tenant_profile(id),
  title text NOT NULL,
  region text,
  country text,
  state text,
  city text,
  address text,
  zip text,
  phone text,
  email text,
  additional_info jsonb,
  version integer NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS users (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  tenant_id uuid REFERENCES tenant(id) ON DELETE CASCADE,
  customer_id uuid,
  authority text NOT NULL CHECK (authority IN ('SYS_ADMIN','TENANT_ADMIN','CUSTOMER_USER')),
  email text NOT NULL,
  first_name text,
  last_name text,
  phone text,
  additional_info jsonb,
  version integer NOT NULL DEFAULT 1,
  UNIQUE (email)
);

CREATE TABLE IF NOT EXISTS user_credentials (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  password_hash text,
  enabled boolean NOT NULL DEFAULT true,
  activate_token_hash text,
  reset_token_hash text,
  token_expires_at bigint,
  failed_attempts integer NOT NULL DEFAULT 0,
  locked_until bigint,
  password_changed_time bigint,
  last_login_time bigint,
  token_version integer NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS device_profile (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  tenant_id uuid NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  name text NOT NULL,
  description text,
  image text,
  is_default boolean NOT NULL DEFAULT false,
  transport_type text NOT NULL DEFAULT 'DEFAULT',
  provision_type text NOT NULL DEFAULT 'DISABLED',
  provision_device_key text,
  default_rule_chain_id uuid,
  default_edge_rule_chain_id uuid,
  default_dashboard_id uuid,
  default_queue_name text NOT NULL DEFAULT 'main',
  firmware_id uuid,
  software_id uuid,
  profile_data jsonb NOT NULL DEFAULT '{}'::jsonb,
  schema_version smallint NOT NULL DEFAULT 1,
  version integer NOT NULL DEFAULT 1,
  UNIQUE (tenant_id, name)
);

CREATE TABLE IF NOT EXISTS device (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  tenant_id uuid NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  customer_id uuid,
  device_profile_id uuid NOT NULL REFERENCES device_profile(id),
  name text NOT NULL,
  label text,
  is_gateway boolean NOT NULL DEFAULT false,
  firmware_id uuid,
  software_id uuid,
  external_id text,
  additional_info jsonb,
  tags text[] NOT NULL DEFAULT '{}',
  version integer NOT NULL DEFAULT 1,
  UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS device_tenant_idx ON device (tenant_id);
CREATE INDEX IF NOT EXISTS device_profile_idx ON device (tenant_id, device_profile_id);

CREATE TABLE IF NOT EXISTS device_credentials (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  tenant_id uuid NOT NULL,
  device_id uuid NOT NULL UNIQUE REFERENCES device(id) ON DELETE CASCADE,
  credentials_type text NOT NULL,
  lookup_id bytea NOT NULL UNIQUE,
  credentials_enc bytea,
  basic_password_hash bytea
);

CREATE INDEX IF NOT EXISTS device_cred_lookup_idx ON device_credentials (lookup_id);

CREATE TABLE IF NOT EXISTS key_dictionary (
  key text PRIMARY KEY,
  key_id integer GENERATED ALWAYS AS IDENTITY UNIQUE
);

CREATE TABLE IF NOT EXISTS ts_kv (
  entity_id uuid NOT NULL,
  key_id integer NOT NULL,
  ts bigint NOT NULL,
  bool_v boolean,
  str_v text,
  long_v bigint,
  dbl_v double precision,
  json_v jsonb,
  PRIMARY KEY (entity_id, key_id, ts)
);

CREATE TABLE IF NOT EXISTS ts_kv_latest (
  entity_id uuid NOT NULL,
  key_id integer NOT NULL,
  ts bigint NOT NULL,
  bool_v boolean,
  str_v text,
  long_v bigint,
  dbl_v double precision,
  json_v jsonb,
  PRIMARY KEY (entity_id, key_id)
);

-- Row-Level Security (RLS) policies for multi-tenancy defense-in-depth
ALTER TABLE device ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS device_tenant_isolation ON device;
CREATE POLICY device_tenant_isolation ON device
  FOR ALL
  USING (
    current_setting('app.tenant_id', true) IS NULL OR
    current_setting('app.tenant_id', true) = '' OR
    tenant_id::text = current_setting('app.tenant_id', true)
  );

ALTER TABLE device_credentials ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS device_cred_tenant_isolation ON device_credentials;
CREATE POLICY device_cred_tenant_isolation ON device_credentials
  FOR ALL
  USING (
    current_setting('app.tenant_id', true) IS NULL OR
    current_setting('app.tenant_id', true) = '' OR
    tenant_id::text = current_setting('app.tenant_id', true)
  );

-- +goose Down
DROP POLICY IF EXISTS device_cred_tenant_isolation ON device_credentials;
DROP POLICY IF EXISTS device_tenant_isolation ON device;
DROP TABLE IF EXISTS ts_kv_latest;
DROP TABLE IF EXISTS ts_kv;
DROP TABLE IF EXISTS key_dictionary;
DROP TABLE IF EXISTS device_credentials;
DROP TABLE IF EXISTS device;
DROP TABLE IF EXISTS device_profile;
DROP TABLE IF EXISTS user_credentials;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS tenant;
DROP TABLE IF EXISTS tenant_profile;
