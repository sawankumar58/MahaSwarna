package com.mahaswarna.local.migration

import androidx.room.migration.Migration
import androidx.sqlite.db.SupportSQLiteDatabase

/**
 * Room database migrations.
 *
 * MIGRATION POLICY — NON-NEGOTIABLE:
 * - NEVER call .fallbackToDestructiveMigration() on the DB builder.
 * - Diary data (bills, customers, ledger_entries) is local-only and unrecoverable.
 *   A destructive migration silently wipes a jeweller's entire transaction history.
 * - Every schema bump MUST have an explicit Migration here AND a @MigrationTest
 *   asserting that Diary row counts are identical before and after.
 *
 * Registration in DatabaseModule:
 * ```kotlin
 * Room.databaseBuilder(context, AppDatabase::class.java, "mahaswarna.db")
 *     .addMigrations(MIGRATION_1_2)
 *     .build()
 * ```
 */
object Migrations {

    /**
     * v1 → v2: Adds Phase 2 Diary tables.
     *
     * Phase 1 tables (rates, home, alerts, designs) are NOT modified.
     * All new tables are additive — zero risk to existing session data.
     *
     * Diary tables added:
     *   bills               — one row per invoice generated
     *   bill_fts            — content-backed FTS4 index on customerName + itemsSummary
     *   customers           — customer address book
     *   customer_fts        — content-backed FTS4 index on name
     *   ledger_entries      — debit/credit ledger per customer
     */
    val MIGRATION_1_2 = object : Migration(1, 2) {
        override fun migrate(db: SupportSQLiteDatabase) {

            // ── bills ────────────────────────────────────────────────────────
            db.execSQL("""
                CREATE TABLE IF NOT EXISTS `bills` (
                    `id`             TEXT    NOT NULL,
                    `customerId`     TEXT    NOT NULL DEFAULT '',
                    `customerName`   TEXT    NOT NULL,
                    `customerPhone`  TEXT    NOT NULL DEFAULT '',
                    `itemsSummary`   TEXT    NOT NULL,
                    `metalType`      TEXT    NOT NULL,
                    `totalWeightG`   REAL    NOT NULL,
                    `metalValueInr`  REAL    NOT NULL,
                    `makingCharges`  REAL    NOT NULL DEFAULT 0.0,
                    `totalInr`       REAL    NOT NULL,
                    `paymentMode`    TEXT    NOT NULL DEFAULT 'cash',
                    `notes`          TEXT    NOT NULL DEFAULT '',
                    `goldRateUsed`   REAL    NOT NULL,
                    `silverRateUsed` REAL    NOT NULL DEFAULT 0.0,
                    `rateSource`     TEXT    NOT NULL DEFAULT 'live',
                    `createdAt`      INTEGER NOT NULL,
                    `syncedAt`       INTEGER,
                    PRIMARY KEY(`id`)
                )
            """.trimIndent())

            // ── bill_fts (content-backed FTS4) ───────────────────────────────
            // contentEntity = bills ensures Room auto-updates the FTS virtual table.
            db.execSQL("""
                CREATE VIRTUAL TABLE IF NOT EXISTS `bill_fts`
                    USING fts4(content=`bills`, `customerName`, `itemsSummary`)
            """.trimIndent())

            // ── customers ────────────────────────────────────────────────────
            db.execSQL("""
                CREATE TABLE IF NOT EXISTS `customers` (
                    `id`        TEXT    NOT NULL,
                    `name`      TEXT    NOT NULL,
                    `phone`     TEXT    NOT NULL DEFAULT '',
                    `address`   TEXT    NOT NULL DEFAULT '',
                    `gstNumber` TEXT    NOT NULL DEFAULT '',
                    `notes`     TEXT    NOT NULL DEFAULT '',
                    `createdAt` INTEGER NOT NULL,
                    `updatedAt` INTEGER NOT NULL,
                    PRIMARY KEY(`id`)
                )
            """.trimIndent())

            // ── customer_fts ─────────────────────────────────────────────────
            db.execSQL("""
                CREATE VIRTUAL TABLE IF NOT EXISTS `customer_fts`
                    USING fts4(content=`customers`, `name`)
            """.trimIndent())

            // ── ledger_entries ───────────────────────────────────────────────
            db.execSQL("""
                CREATE TABLE IF NOT EXISTS `ledger_entries` (
                    `id`          TEXT    NOT NULL,
                    `customerId`  TEXT    NOT NULL,
                    `billId`      TEXT,
                    `type`        TEXT    NOT NULL,
                    `amountInr`   REAL    NOT NULL,
                    `description` TEXT    NOT NULL DEFAULT '',
                    `createdAt`   INTEGER NOT NULL,
                    PRIMARY KEY(`id`),
                    FOREIGN KEY(`customerId`) REFERENCES `customers`(`id`) ON DELETE CASCADE
                )
            """.trimIndent())

            // ── indices ──────────────────────────────────────────────────────
            db.execSQL("CREATE INDEX IF NOT EXISTS `idx_bills_customer_id`  ON `bills`(`customerId`)")
            db.execSQL("CREATE INDEX IF NOT EXISTS `idx_bills_created_at`   ON `bills`(`createdAt`)")
            db.execSQL("CREATE INDEX IF NOT EXISTS `idx_ledger_customer_id` ON `ledger_entries`(`customerId`)")
            db.execSQL("CREATE INDEX IF NOT EXISTS `idx_ledger_created_at`  ON `ledger_entries`(`createdAt`)")
        }
    }
}
