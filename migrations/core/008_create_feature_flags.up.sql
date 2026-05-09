-- feature_flags: key-value store for runtime configuration
-- CRITICAL: ALL keys must be seeded here. Missing keys cause clients to use
-- hardcoded defaults — kill_switch_image_search defaults to FALSE if absent,
-- enabling an unimplemented backend endpoint on fresh install.
CREATE TABLE IF NOT EXISTS feature_flags (
  key        TEXT        PRIMARY KEY,
  value      TEXT        NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER trg_feature_flags_updated_at
  BEFORE UPDATE ON feature_flags
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- pg NOTIFY on flag change (consumed by gateway flags_repository.go)
CREATE OR REPLACE FUNCTION notify_flag_change()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  PERFORM pg_notify('flag_updated',
    json_build_object('key', NEW.key, 'value', NEW.value)::text);
  RETURN NEW;
END; $$;

CREATE TRIGGER trg_flag_notify
  AFTER INSERT OR UPDATE ON feature_flags
  FOR EACH ROW EXECUTE FUNCTION notify_flag_change();

-- ── Required seed data ────────────────────────────────────────────────────
-- ON CONFLICT DO NOTHING: safe to re-run; does not overwrite live values.

-- Feature toggles
INSERT INTO feature_flags (key, value) VALUES
  ('ai_enabled',       'true'),
  ('shop_enabled',     'true'),
  ('ws_enabled',       'true'),
  ('payments_enabled', 'true'),
  ('catalog_enabled',  'true')
ON CONFLICT (key) DO NOTHING;

-- Kill switches
-- IMPORTANT: kill_switch_image_search MUST default to 'true'.
-- POST /catalog/image-search is not yet implemented. Do not set to 'false'
-- until the backend Vision pipeline ships in the same release as the Android toggle.
INSERT INTO feature_flags (key, value) VALUES
  ('kill_switch_ai',           'false'),
  ('kill_switch_ws',           'false'),
  ('kill_switch_payments',     'false'),
  ('kill_switch_catalog',      'false'),
  ('kill_switch_image_search', 'true')
ON CONFLICT (key) DO NOTHING;

-- Numeric params (read by watchdog + rate limiter — must be present at startup)
-- rate_sanity_threshold_pct: max delta % between consecutive Gemini snapshots
-- rate_limit_bff_free_rpm: FREE-tier RPM cap for GET /bff/home
-- NOTE: raise rate_limit_bff_free_rpm to 60 BEFORE setting kill_switch_ws to 'true'
--       (see scripts/activate_ws_killswitch.sh)
INSERT INTO feature_flags (key, value) VALUES
  ('rate_sanity_threshold_pct', '2.0'),
  ('rate_limit_bff_free_rpm',   '40')
ON CONFLICT (key) DO NOTHING;
