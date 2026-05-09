CREATE TABLE IF NOT EXISTS alerts (
  id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  city_id      TEXT         NOT NULL,
  metal        TEXT         NOT NULL CHECK (metal IN ('gold', 'silver')),
  threshold    NUMERIC(12,2) NOT NULL CHECK (threshold > 0),
  direction    TEXT         NOT NULL CHECK (direction IN ('above', 'below')),
  created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  delivered_at TIMESTAMPTZ                           -- null = not yet triggered
);

CREATE INDEX idx_alerts_user_id   ON alerts(user_id);
CREATE INDEX idx_alerts_city_metal ON alerts(city_id, metal) WHERE delivered_at IS NULL;
-- alert_threshold_job.go queries: WHERE city_id=$1 AND metal=$2 AND delivered_at IS NULL
