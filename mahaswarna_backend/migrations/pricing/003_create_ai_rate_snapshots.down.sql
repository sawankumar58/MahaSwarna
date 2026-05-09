DROP TRIGGER IF EXISTS trg_rate_snapshot_notify ON ai_rate_snapshots;
DROP FUNCTION IF EXISTS notify_rate_snapshot();
DROP TABLE IF EXISTS ai_rate_snapshots;
