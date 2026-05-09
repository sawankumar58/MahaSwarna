-- consent_log: insert-only audit trail of user consent events
-- REVOKE UPDATE, DELETE enforces immutability at the DB layer.
-- GDPR/DPDPA: on account deletion, consent_log rows are RETAINED for legal compliance.
-- The app_role cannot delete them; only a DBA with elevated privileges can.
CREATE TABLE IF NOT EXISTS consent_log (
  id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  consent_type TEXT        NOT NULL,   -- ALLOWLIST: privacy_policy | tos (enforced in log_consent_usecase.go)
  version      TEXT        NOT NULL,   -- policy version string, e.g. "1.0"
  consented_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(user_id, consent_type, version)  -- idempotency: re-consent is a no-op
);

CREATE INDEX idx_consent_log_user_id ON consent_log(user_id);

-- Enforce immutability: app_role cannot mutate consent records
REVOKE UPDATE, DELETE ON consent_log FROM app_role;
