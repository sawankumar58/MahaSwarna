CREATE TABLE IF NOT EXISTS device_tokens (
  id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id  TEXT        NOT NULL,   -- stable device identifier
  token      TEXT        NOT NULL,   -- FCM registration token
  platform   TEXT        NOT NULL DEFAULT 'android',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(user_id, device_id)   -- upsert target
);

CREATE INDEX idx_device_tokens_user_id ON device_tokens(user_id);

CREATE TRIGGER trg_device_tokens_updated_at
  BEFORE UPDATE ON device_tokens
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
