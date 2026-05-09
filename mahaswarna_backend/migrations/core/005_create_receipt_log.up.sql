-- receipt_log: immutable audit trail for Google Play IAP verifications
-- Each verification attempt is a new row. Status cannot be updated post-insert.
-- The subscriptions table is the authoritative subscription state.
CREATE TABLE IF NOT EXISTS receipt_log (
  id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id        UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  purchase_token TEXT        NOT NULL UNIQUE,   -- idempotency: INSERT ON CONFLICT DO NOTHING
  product_id     TEXT        NOT NULL,
  package_name   TEXT        NOT NULL,
  status         TEXT        NOT NULL,           -- PENDING | VERIFIED | FAILED
  play_api_response JSONB,                       -- raw Google Play API response (redact PII before storing)
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_receipt_log_user_id       ON receipt_log(user_id);
CREATE INDEX idx_receipt_log_purchase_token ON receipt_log(purchase_token);

-- Append-only: app_role cannot mutate receipt records
REVOKE UPDATE, DELETE ON receipt_log FROM app_role;
