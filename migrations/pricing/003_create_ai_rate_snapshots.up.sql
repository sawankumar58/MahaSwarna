-- ai_rate_snapshots: Gemini AI rate snapshots (sole rate source in v1)
-- TTL: 30 days. Rows older than 30 days are purged by cleanup_old_data.sh.
-- Redis cache (TTL 1h) is the primary read path; DB is fallback.
-- Stale flag: set by rate_quality_watchdog.go when delta > rate_sanity_threshold_pct
--             OR when snapshot is outside IST trading window (Mon-Sat 10:00-19:00 IST).
CREATE TABLE IF NOT EXISTS ai_rate_snapshots (
  id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
  city_id      TEXT         NOT NULL REFERENCES cities(id),
  gold         NUMERIC(12,2) NOT NULL CHECK (gold > 0),
  silver       NUMERIC(12,2) NOT NULL CHECK (silver > 0),
  source       TEXT         NOT NULL DEFAULT 'gemini',
  is_stale     BOOLEAN      NOT NULL DEFAULT FALSE,
  generated_at TIMESTAMPTZ  NOT NULL,   -- IST timestamp of the Gemini query
  created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ai_snapshots_city_generated ON ai_rate_snapshots(city_id, generated_at DESC);
CREATE INDEX idx_ai_snapshots_cleanup        ON ai_rate_snapshots(created_at)
  WHERE created_at < NOW() - INTERVAL '30 days';

-- pg NOTIFY on new snapshot (consumed by ws/redis_fanout.go → WS clients)
CREATE OR REPLACE FUNCTION notify_rate_snapshot()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  PERFORM pg_notify('ai_rate_snapshot_ready',
    json_build_object(
      'city_id',      NEW.city_id,
      'gold',         NEW.gold,
      'silver',       NEW.silver,
      'source',       NEW.source,
      'is_stale',     NEW.is_stale,
      'generated_at', NEW.generated_at
    )::text
  );
  RETURN NEW;
END; $$;

CREATE TRIGGER trg_rate_snapshot_notify
  AFTER INSERT ON ai_rate_snapshots
  FOR EACH ROW EXECUTE FUNCTION notify_rate_snapshot();
