-- design_catalog: jewellery designs for the catalog feature
-- Full-text search: PostgreSQL tsvector GIN index on title + description + tags.
-- view_count: incremented via Redis INCR, flushed every 5 min by flush_view_counts_job.go.
--             DO NOT increment in DB on every request — use Redis buffer.
CREATE TABLE IF NOT EXISTS design_catalog (
  id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  title          TEXT        NOT NULL,
  description    TEXT        NOT NULL,
  category       TEXT        NOT NULL,
  style          TEXT        NOT NULL,
  region         TEXT,                              -- NULL = applicable to all regions
  metal_type     TEXT        NOT NULL CHECK (metal_type IN ('gold', 'silver', 'both')),
  image_url      TEXT        NOT NULL,
  tags           TEXT[]      NOT NULL DEFAULT '{}',
  view_count     BIGINT      NOT NULL DEFAULT 0,
  search_vector  TSVECTOR
    GENERATED ALWAYS AS (
      to_tsvector('english',
        coalesce(title, '') || ' ' ||
        coalesce(description, '') || ' ' ||
        coalesce(array_to_string(tags, ' '), '')
      )
    ) STORED,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- GIN index for full-text search (catalog/search endpoint)
CREATE INDEX idx_design_catalog_search  ON design_catalog USING GIN(search_vector);

-- B-tree indexes for sort/filter operations
CREATE INDEX idx_design_catalog_region      ON design_catalog(region) WHERE region IS NOT NULL;
CREATE INDEX idx_design_catalog_view_count  ON design_catalog(view_count DESC);
CREATE INDEX idx_design_catalog_metal_type  ON design_catalog(metal_type);

CREATE TRIGGER trg_design_catalog_updated_at
  BEFORE UPDATE ON design_catalog
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- NOTE: set_updated_at() is defined in core migration 001.
-- intelligence service MUST run AFTER core migrations.
