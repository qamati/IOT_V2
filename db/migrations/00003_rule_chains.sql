-- +goose Up
-- Migration 00003: Rule Chains, Revisions, and Node Catalog Persistence

CREATE TABLE IF NOT EXISTS rule_chain (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  tenant_id uuid NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  name text NOT NULL,
  type text NOT NULL DEFAULT 'CORE',
  is_root boolean NOT NULL DEFAULT false,
  published_revision integer NOT NULL DEFAULT 1,
  additional_info jsonb NOT NULL DEFAULT '{}'::jsonb,
  version integer NOT NULL DEFAULT 1,
  UNIQUE (tenant_id, type, name)
);
CREATE INDEX IF NOT EXISTS rule_chain_tenant_idx ON rule_chain (tenant_id, is_root);

CREATE TABLE IF NOT EXISTS rule_chain_revision (
  rule_chain_id uuid NOT NULL REFERENCES rule_chain(id) ON DELETE CASCADE,
  revision integer NOT NULL,
  status text NOT NULL DEFAULT 'PUBLISHED' CHECK (status IN ('DRAFT','PUBLISHED','ARCHIVED')),
  definition jsonb NOT NULL,
  author_id uuid,
  comment text,
  created_time bigint NOT NULL,
  PRIMARY KEY (rule_chain_id, revision)
);

-- RLS policies for rule chains
ALTER TABLE rule_chain ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS rule_chain_tenant_isolation ON rule_chain;
CREATE POLICY rule_chain_tenant_isolation ON rule_chain
  FOR ALL
  USING (
    current_setting('app.tenant_id', true) IS NULL OR
    current_setting('app.tenant_id', true) = '' OR
    tenant_id::text = current_setting('app.tenant_id', true)
  );

-- +goose Down
DROP POLICY IF EXISTS rule_chain_tenant_isolation ON rule_chain;
DROP TABLE IF EXISTS rule_chain_revision;
DROP TABLE IF EXISTS rule_chain;
