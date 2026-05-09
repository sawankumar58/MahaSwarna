-- Create app_role for least-privilege access
-- app_role is the DB user the services connect as.
DO $$ BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'app_role') THEN
    CREATE ROLE app_role;
  END IF;
END $$;

GRANT CONNECT ON DATABASE mahaswarna TO app_role;
GRANT USAGE ON SCHEMA public TO app_role;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_role;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT USAGE, SELECT ON SEQUENCES TO app_role;

CREATE TABLE IF NOT EXISTS users (
  id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  phone           TEXT        UNIQUE NOT NULL,              -- E.164: +91XXXXXXXXXX
  city_id         TEXT        NOT NULL,                     -- slug from cities table
  tier            TEXT        NOT NULL DEFAULT 'FREE',      -- FREE | PREMIUM | ADMIN
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at      TIMESTAMPTZ,                              -- soft delete
  hard_deleted_at TIMESTAMPTZ                               -- set by hard_delete_job.go after 30d grace
);

CREATE INDEX idx_users_phone      ON users(phone);
CREATE INDEX idx_users_deleted_at ON users(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX idx_users_hard_delete ON users(deleted_at)
  WHERE deleted_at IS NOT NULL AND hard_deleted_at IS NULL;

-- Trigger: auto-update updated_at
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN NEW.updated_at = NOW(); RETURN NEW; END; $$;

CREATE TRIGGER trg_users_updated_at
  BEFORE UPDATE ON users
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
