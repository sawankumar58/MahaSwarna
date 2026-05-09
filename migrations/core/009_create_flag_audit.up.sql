-- flag_audit: immutable change log for feature flag mutations
-- Retention: 1 year (cleanup_old_data.sh purges older rows)
CREATE TABLE IF NOT EXISTS flag_audit (
  id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  key        TEXT        NOT NULL,
  old_value  TEXT,
  new_value  TEXT        NOT NULL,
  changed_by TEXT        NOT NULL,   -- user_id (admin) or 'system'
  changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_flag_audit_key        ON flag_audit(key);
CREATE INDEX idx_flag_audit_changed_at ON flag_audit(changed_at);
