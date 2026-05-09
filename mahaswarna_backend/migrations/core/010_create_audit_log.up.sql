-- audit_log: append-only compliance log for sensitive operations
-- Critical entries: account_deleted, hard_deleted, consent_overridden, admin_flag_changed
CREATE TABLE IF NOT EXISTS audit_log (
  id        UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  actor     TEXT        NOT NULL,   -- user_id, service name, or 'system'
  action    TEXT        NOT NULL,   -- e.g. 'account_deleted', 'subscription_activated'
  entity    TEXT        NOT NULL,   -- table name, e.g. 'users', 'subscriptions'
  entity_id TEXT,                   -- primary key of the affected row
  metadata  JSONB,                  -- contextual data (never include PII, tokens, or secrets)
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_log_actor      ON audit_log(actor);
CREATE INDEX idx_audit_log_action     ON audit_log(action);
CREATE INDEX idx_audit_log_created_at ON audit_log(created_at DESC);

-- Append-only: app_role cannot mutate audit entries
REVOKE UPDATE, DELETE ON audit_log FROM app_role;
