-- +goose Up
-- Migration 00004: Alarm Rules, Notification Targets, Templates, Rules, and Inbox

-- 1. Alarm Rules (Profile & Entity-level declarative alert rules)
CREATE TABLE IF NOT EXISTS alarm_rule (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  tenant_id uuid NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  name text NOT NULL,
  scope text NOT NULL DEFAULT 'DEVICE_PROFILE' CHECK (scope IN ('DEVICE_PROFILE','DEVICE','ASSET','CUSTOMER')),
  target_id uuid,
  alarm_type text NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  create_rules jsonb NOT NULL DEFAULT '{}'::jsonb,
  clear_rule jsonb,
  propagate jsonb NOT NULL DEFAULT '{"toOwner": true, "toTenant": true}'::jsonb,
  additional_info jsonb NOT NULL DEFAULT '{}'::jsonb,
  version integer NOT NULL DEFAULT 1,
  UNIQUE (tenant_id, name)
);
CREATE INDEX IF NOT EXISTS alarm_rule_tenant_idx ON alarm_rule (tenant_id, enabled);

-- 2. Notification Targets (Recipients, Email Lists, Slack Webhooks, Webhooks)
CREATE TABLE IF NOT EXISTS notification_target (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  tenant_id uuid NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  name text NOT NULL,
  type text NOT NULL CHECK (type IN ('PLATFORM_USERS','EMAIL','SLACK','WEBHOOK','SMS')),
  config jsonb NOT NULL DEFAULT '{}'::jsonb,
  UNIQUE (tenant_id, name)
);
CREATE INDEX IF NOT EXISTS notification_target_tenant_idx ON notification_target (tenant_id);

-- 3. Notification Templates (Channel layouts, templates, and buttons)
CREATE TABLE IF NOT EXISTS notification_template (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  tenant_id uuid NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  name text NOT NULL,
  type text NOT NULL DEFAULT 'ALARM',
  config jsonb NOT NULL DEFAULT '{}'::jsonb,
  UNIQUE (tenant_id, name)
);
CREATE INDEX IF NOT EXISTS notification_template_tenant_idx ON notification_template (tenant_id);

-- 4. Notification Rules (Trigger mapping + escalation chains)
CREATE TABLE IF NOT EXISTS notification_rule (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  tenant_id uuid NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  name text NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  template_id uuid NOT NULL REFERENCES notification_template(id) ON DELETE CASCADE,
  trigger_type text NOT NULL DEFAULT 'ALARM',
  trigger_config jsonb NOT NULL DEFAULT '{}'::jsonb,
  recipients_config jsonb NOT NULL DEFAULT '[]'::jsonb,
  throttle jsonb,
  UNIQUE (tenant_id, name)
);
CREATE INDEX IF NOT EXISTS notification_rule_tenant_idx ON notification_rule (tenant_id, enabled);

-- 5. Notification Requests & Deliveries
CREATE TABLE IF NOT EXISTS notification_request (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  tenant_id uuid NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  rule_id uuid REFERENCES notification_rule(id) ON DELETE SET NULL,
  template_id uuid REFERENCES notification_template(id) ON DELETE SET NULL,
  targets uuid[] NOT NULL DEFAULT '{}',
  info jsonb NOT NULL DEFAULT '{}'::jsonb,
  status text NOT NULL DEFAULT 'SENT',
  stats jsonb NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX IF NOT EXISTS notification_req_tenant_idx ON notification_request (tenant_id, created_time DESC);

CREATE TABLE IF NOT EXISTS notification_delivery (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  tenant_id uuid NOT NULL,
  request_id uuid REFERENCES notification_request(id) ON DELETE CASCADE,
  channel text NOT NULL,
  recipient text NOT NULL,
  status text NOT NULL DEFAULT 'SENT',
  last_error text
);
CREATE INDEX IF NOT EXISTS notification_delivery_req_idx ON notification_delivery (request_id);

-- 6. In-App User Inbox Notifications
CREATE TABLE IF NOT EXISTS notification (
  id uuid PRIMARY KEY,
  created_time bigint NOT NULL,
  tenant_id uuid NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  recipient_id uuid NOT NULL,
  request_id uuid REFERENCES notification_request(id) ON DELETE CASCADE,
  type text NOT NULL DEFAULT 'ALARM',
  subject text NOT NULL,
  body text NOT NULL,
  info jsonb NOT NULL DEFAULT '{}'::jsonb,
  status text NOT NULL DEFAULT 'UNREAD' CHECK (status IN ('UNREAD', 'READ'))
);
CREATE INDEX IF NOT EXISTS notification_inbox_idx ON notification (tenant_id, recipient_id, status, created_time DESC);

-- RLS Tenant Isolation Policies
ALTER TABLE alarm_rule ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS alarm_rule_tenant_isolation ON alarm_rule;
CREATE POLICY alarm_rule_tenant_isolation ON alarm_rule
  FOR ALL USING (
    current_setting('app.tenant_id', true) IS NULL OR
    current_setting('app.tenant_id', true) = '' OR
    tenant_id::text = current_setting('app.tenant_id', true)
  );

ALTER TABLE notification_target ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS notification_target_tenant_isolation ON notification_target;
CREATE POLICY notification_target_tenant_isolation ON notification_target
  FOR ALL USING (
    current_setting('app.tenant_id', true) IS NULL OR
    current_setting('app.tenant_id', true) = '' OR
    tenant_id::text = current_setting('app.tenant_id', true)
  );

ALTER TABLE notification_template ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS notification_template_tenant_isolation ON notification_template;
CREATE POLICY notification_template_tenant_isolation ON notification_template
  FOR ALL USING (
    current_setting('app.tenant_id', true) IS NULL OR
    current_setting('app.tenant_id', true) = '' OR
    tenant_id::text = current_setting('app.tenant_id', true)
  );

ALTER TABLE notification_rule ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS notification_rule_tenant_isolation ON notification_rule;
CREATE POLICY notification_rule_tenant_isolation ON notification_rule
  FOR ALL USING (
    current_setting('app.tenant_id', true) IS NULL OR
    current_setting('app.tenant_id', true) = '' OR
    tenant_id::text = current_setting('app.tenant_id', true)
  );

ALTER TABLE notification ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS notification_tenant_isolation ON notification;
CREATE POLICY notification_tenant_isolation ON notification
  FOR ALL USING (
    current_setting('app.tenant_id', true) IS NULL OR
    current_setting('app.tenant_id', true) = '' OR
    tenant_id::text = current_setting('app.tenant_id', true)
  );

-- +goose Down
DROP POLICY IF EXISTS notification_tenant_isolation ON notification;
DROP TABLE IF EXISTS notification;
DROP TABLE IF EXISTS notification_delivery;
DROP TABLE IF EXISTS notification_request;
DROP POLICY IF EXISTS notification_rule_tenant_isolation ON notification_rule;
DROP TABLE IF EXISTS notification_rule;
DROP POLICY IF EXISTS notification_template_tenant_isolation ON notification_template;
DROP TABLE IF EXISTS notification_template;
DROP POLICY IF EXISTS notification_target_tenant_isolation ON notification_target;
DROP TABLE IF EXISTS notification_target;
DROP POLICY IF EXISTS alarm_rule_tenant_isolation ON alarm_rule;
DROP TABLE IF EXISTS alarm_rule;
