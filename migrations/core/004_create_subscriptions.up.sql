CREATE TABLE IF NOT EXISTS subscriptions (
  id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id        UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  tier           TEXT        NOT NULL,                         -- PREMIUM | ADMIN
  purchase_token TEXT        UNIQUE,                           -- Google Play purchase token
  product_id     TEXT,
  package_name   TEXT        NOT NULL DEFAULT 'com.mahaswarna',
  status         TEXT        NOT NULL DEFAULT 'ACTIVE',        -- ACTIVE | EXPIRED | CANCELLED
  activated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at     TIMESTAMPTZ,                                  -- null for lifetime/admin tiers
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_subscriptions_user_id     ON subscriptions(user_id);
CREATE INDEX idx_subscriptions_status      ON subscriptions(status) WHERE status = 'ACTIVE';
CREATE INDEX idx_subscriptions_expires_at  ON subscriptions(expires_at)
  WHERE expires_at IS NOT NULL AND status = 'ACTIVE';

CREATE TRIGGER trg_subscriptions_updated_at
  BEFORE UPDATE ON subscriptions
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- pg NOTIFY on subscription state change (consumed by pricing + intelligence)
CREATE OR REPLACE FUNCTION notify_subscription_change()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'INSERT' OR OLD.status IS DISTINCT FROM NEW.status THEN
    PERFORM pg_notify(
      'subscription_changed',
      json_build_object('user_id', NEW.user_id, 'tier', NEW.tier, 'status', NEW.status)::text
    );
  END IF;
  RETURN NEW;
END; $$;

CREATE TRIGGER trg_subscription_notify
  AFTER INSERT OR UPDATE ON subscriptions
  FOR EACH ROW EXECUTE FUNCTION notify_subscription_change();
