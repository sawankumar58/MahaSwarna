-- invoices: GST-aware PDF invoice records
-- ADR-001 DECIDED: PDF bytes are NOT stored server-side.
-- There is intentionally no column for the PDF S3 key or blob.
-- Only metadata is persisted. Client is responsible for saving the PDF locally.
-- pdf_size_bytes is retained for audit and quota tracking only.
-- Daily invoice limit: 60 per shop per IST day (enforced via Redis counter in
--   invoice_handler.go; key: invoice_count:{shopID}:{YYYY-MM-DD-IST}).
CREATE TABLE IF NOT EXISTS invoices (
  id                   UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
  shop_id              UUID          NOT NULL REFERENCES shops(id),
  user_id              UUID          NOT NULL,                  -- denormalised
  customer_name        TEXT          NOT NULL,
  customer_phone       TEXT,                                    -- optional
  items                JSONB         NOT NULL,                  -- []InvoiceLineItem
  payment_mode         TEXT          NOT NULL CHECK (payment_mode IN ('cash', 'upi', 'card')),
  notes                TEXT,
  gold_rate_override   NUMERIC(12,2),
  silver_rate_override NUMERIC(12,2),
  rate_source          TEXT          NOT NULL,                  -- live | stale | client_override | manual_override
  pdf_size_bytes       INTEGER,                                 -- audit only — no PDF stored (ADR-001)
  generated_at         TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- Primary query pattern: invoice history for a shop, newest first
CREATE INDEX idx_invoices_shop_generated ON invoices(shop_id, generated_at DESC);
CREATE INDEX idx_invoices_user_id        ON invoices(user_id);
