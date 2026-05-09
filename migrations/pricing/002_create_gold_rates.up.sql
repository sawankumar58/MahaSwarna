-- gold_rates: manual override rates (admin-inserted during Gemini outage)
-- Normal rates come from ai_rate_snapshots.
-- source is always 'manual_override' for rows in this table.
CREATE TABLE IF NOT EXISTS gold_rates (
  id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
  city_id    TEXT         NOT NULL REFERENCES cities(id),
  gold       NUMERIC(12,2) NOT NULL CHECK (gold > 0),
  silver     NUMERIC(12,2) NOT NULL CHECK (silver > 0),
  source     TEXT         NOT NULL DEFAULT 'manual_override',
  is_stale   BOOLEAN      NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_gold_rates_city_created ON gold_rates(city_id, created_at DESC);
