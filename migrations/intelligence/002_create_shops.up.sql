-- shops: jeweller shop profiles (PREMIUM users only)
-- user_id is denormalised TEXT — no cross-schema FK to core.users.
-- Referential integrity is enforced at the application layer via
-- subscription_projection.go Redis read model check before INSERT.
CREATE TABLE IF NOT EXISTS shops (
  id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id           UUID        NOT NULL,   -- core.users.id (no FK — cross-schema)
  name              TEXT        NOT NULL,
  address           TEXT        NOT NULL,
  gst_number        TEXT        NOT NULL,   -- GSTIN: validated by CreateShopRequest (regex)
  phone             TEXT        NOT NULL,
  banner_url        TEXT,                   -- CDN URL post-moderation (null until confirmed)
  banner_object_key TEXT,                   -- S3 object key (for cleanup on banner replace)
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_shops_user_id ON shops(user_id);

CREATE TRIGGER trg_shops_updated_at
  BEFORE UPDATE ON shops
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- pg NOTIFY: consumed by core service for shop-level quota checks
CREATE OR REPLACE FUNCTION notify_shop_registered()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'INSERT' THEN
    PERFORM pg_notify('shop_registered',
      json_build_object('shop_id', NEW.id, 'user_id', NEW.user_id)::text);
  END IF;
  RETURN NEW;
END; $$;

CREATE TRIGGER trg_shop_registered_notify
  AFTER INSERT ON shops
  FOR EACH ROW EXECUTE FUNCTION notify_shop_registered();
