-- Room DB migration v1 → v2
-- Adds Phase 2 Diary tables: bills, bill_fts, customers, customer_fts, ledger_entries.
--
-- MIGRATION POLICY — NON-NEGOTIABLE:
--   fallbackToDestructiveMigration() is BANNED.
--   Diary data is local-only and unrecoverable — a destructive migration silently
--   wipes a jeweller's entire transaction history with no recourse.
--   This file is the source of truth. The Room Migration object in Migrations.kt
--   must execute exactly these statements in this order.
--
-- Schema version: 1 → 2
-- Phase 1 tables (rates, home, alerts, designs) are NOT modified — zero downtime.

-- ── bills ──────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS `bills` (
    `id`            TEXT    NOT NULL,
    `customerId`    TEXT    NOT NULL DEFAULT '',
    `customerName`  TEXT    NOT NULL,
    `customerPhone` TEXT    NOT NULL DEFAULT '',
    `itemsSummary`  TEXT    NOT NULL,          -- JSON array of InvoiceLineItem
    `metalType`     TEXT    NOT NULL,          -- "gold" | "silver" | "mixed"
    `totalWeightG`  REAL    NOT NULL,
    `metalValueInr` REAL    NOT NULL,
    `makingCharges` REAL    NOT NULL DEFAULT 0.0,
    `totalInr`      REAL    NOT NULL,
    `paymentMode`   TEXT    NOT NULL DEFAULT 'cash',
    `notes`         TEXT    NOT NULL DEFAULT '',
    `goldRateUsed`  REAL    NOT NULL,          -- per-gram rate at time of bill
    `silverRateUsed` REAL   NOT NULL DEFAULT 0.0,
    `rateSource`    TEXT    NOT NULL DEFAULT 'live',
    `createdAt`     INTEGER NOT NULL,          -- epoch ms (local device time)
    `syncedAt`      INTEGER,                   -- NULL = not synced (Diary is local-only; reserved)
    PRIMARY KEY(`id`)
);

-- ── bill_fts (content-backed FTS4 — Room keeps in sync automatically) ──────────
CREATE VIRTUAL TABLE IF NOT EXISTS `bill_fts`
    USING fts4(content=`bills`, `customerName`, `itemsSummary`);

-- ── customers ──────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS `customers` (
    `id`         TEXT    NOT NULL,
    `name`       TEXT    NOT NULL,
    `phone`      TEXT    NOT NULL DEFAULT '',
    `address`    TEXT    NOT NULL DEFAULT '',
    `gstNumber`  TEXT    NOT NULL DEFAULT '',
    `notes`      TEXT    NOT NULL DEFAULT '',
    `createdAt`  INTEGER NOT NULL,             -- epoch ms
    `updatedAt`  INTEGER NOT NULL,
    PRIMARY KEY(`id`)
);

-- ── customer_fts ───────────────────────────────────────────────────────────────
CREATE VIRTUAL TABLE IF NOT EXISTS `customer_fts`
    USING fts4(content=`customers`, `name`);

-- ── ledger_entries ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS `ledger_entries` (
    `id`          TEXT    NOT NULL,
    `customerId`  TEXT    NOT NULL,
    `billId`      TEXT,                        -- NULL for manual entries not linked to a bill
    `type`        TEXT    NOT NULL,            -- "credit" | "debit"
    `amountInr`   REAL    NOT NULL,
    `description` TEXT    NOT NULL DEFAULT '',
    `createdAt`   INTEGER NOT NULL,            -- epoch ms
    PRIMARY KEY(`id`),
    FOREIGN KEY(`customerId`) REFERENCES `customers`(`id`) ON DELETE CASCADE
);

-- ── indices ────────────────────────────────────────────────────────────────────
CREATE INDEX IF NOT EXISTS `idx_bills_customer_id`     ON `bills`(`customerId`);
CREATE INDEX IF NOT EXISTS `idx_bills_created_at`      ON `bills`(`createdAt`);
CREATE INDEX IF NOT EXISTS `idx_ledger_customer_id`    ON `ledger_entries`(`customerId`);
CREATE INDEX IF NOT EXISTS `idx_ledger_created_at`     ON `ledger_entries`(`createdAt`);
