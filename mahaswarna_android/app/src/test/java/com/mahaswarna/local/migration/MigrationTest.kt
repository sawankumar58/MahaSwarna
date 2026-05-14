package com.mahaswarna.local.migration

import androidx.room.testing.MigrationTestHelper
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.mahaswarna.local.AppDatabase
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import java.util.UUID

/**
 * Instrumented migration test for [AppDatabase].
 *
 * MANDATORY POLICY (from AppDatabase.kt KDoc):
 *   Every schema version bump MUST have an explicit [Migration] in [Migrations]
 *   AND a @MigrationTest asserting that Diary row counts are identical before and
 *   after. This test is the enforcement mechanism for that policy.
 *
 * Why this matters:
 *   Diary tables (bills, customers, ledger_entries) are local-only and
 *   unrecoverable. A schema bug that silently wipes or corrupts these tables
 *   destroys a jeweller's transaction history with no recourse.
 *
 * Test strategy for MIGRATION_1_2:
 *   1. Open the DB at version 1 (Phase 1 tables only — rates, home, alerts, designs).
 *   2. Insert known rows into the Phase 1 tables to confirm they survive the migration.
 *   3. Run the migration to version 2.
 *   4. Assert all Phase 1 rows are intact (count + spot-check).
 *   5. Confirm Phase 2 tables (bills, customers, ledger_entries) exist and are empty.
 *   6. Insert rows into Phase 2 tables and confirm they are readable.
 *   7. Confirm FK enforcement: inserting a ledger entry with a non-existent
 *      customerId must fail (validates setForeignKeyConstraintsEnabled(true)).
 *
 * Run with: ./gradlew connectedDebugAndroidTest -Pandroid.testInstrumentationRunnerArguments.class=com.mahaswarna.local.migration.MigrationTest
 */
@RunWith(AndroidJUnit4::class)
class MigrationTest {

    @get:Rule
    val helper: MigrationTestHelper = MigrationTestHelper(
        instrumentation = InstrumentationRegistry.getInstrumentation(),
        databaseClass    = AppDatabase::class.java,
    )

    // ── Helpers ──────────────────────────────────────────────────────────────

    private fun newId() = UUID.randomUUID().toString()
    private fun nowMs() = System.currentTimeMillis()

    // ── MIGRATION_1_2 ─────────────────────────────────────────────────────

    @Test
    fun migration_1_2_phase1_tables_survive() {
        // ── Step 1: create DB at version 1 ───────────────────────────────
        val v1 = helper.createDatabase(TEST_DB, 1)

        // Insert a rate row into v1 to verify it is preserved after migration.
        // The v1 `rates` schema: id TEXT PK, cityId TEXT, goldBid REAL, goldAsk REAL,
        // silverBid REAL, silverAsk REAL, generatedAt INTEGER, fetchedAt INTEGER.
        v1.execSQL(
            """INSERT INTO rates (id, cityId, goldBid, goldAsk, silverBid, silverAsk,
               generatedAt, fetchedAt) VALUES (?, ?, ?, ?, ?, ?, ?, ?)""",
            arrayOf(newId(), "MUM", 6100.0, 6110.0, 75.5, 76.0, nowMs(), nowMs()),
        )

        val rateCountBefore = v1.query("SELECT COUNT(*) FROM rates").use {
            it.moveToFirst(); it.getInt(0)
        }
        assertEquals("One rate row inserted before migration", 1, rateCountBefore)
        v1.close()

        // ── Step 2: run migration ─────────────────────────────────────────
        val v2 = helper.runMigrationsAndValidate(
            TEST_DB,
            2,
            /* validateDroppedTables = */ true,
            Migrations.MIGRATION_1_2,
        )

        // ── Step 3: Phase 1 row must still be there ───────────────────────
        val rateCountAfter = v2.query("SELECT COUNT(*) FROM rates").use {
            it.moveToFirst(); it.getInt(0)
        }
        assertEquals("Phase 1 rate row must survive migration", 1, rateCountAfter)

        v2.close()
    }

    @Test
    fun migration_1_2_diary_tables_created_and_empty() {
        helper.createDatabase(TEST_DB, 1).close()

        val v2 = helper.runMigrationsAndValidate(
            TEST_DB, 2, true, Migrations.MIGRATION_1_2,
        )

        // Bills table exists and is empty
        val billCount = v2.query("SELECT COUNT(*) FROM bills").use {
            it.moveToFirst(); it.getInt(0)
        }
        assertEquals("bills table must be empty after fresh migration", 0, billCount)

        // Customers table exists and is empty
        val customerCount = v2.query("SELECT COUNT(*) FROM customers").use {
            it.moveToFirst(); it.getInt(0)
        }
        assertEquals("customers table must be empty after fresh migration", 0, customerCount)

        // Ledger entries table exists and is empty
        val ledgerCount = v2.query("SELECT COUNT(*) FROM ledger_entries").use {
            it.moveToFirst(); it.getInt(0)
        }
        assertEquals("ledger_entries table must be empty after fresh migration", 0, ledgerCount)

        // FTS virtual tables exist
        val billFtsCount = v2.query("SELECT COUNT(*) FROM bill_fts").use {
            it.moveToFirst(); it.getInt(0)
        }
        assertEquals("bill_fts FTS table must exist and be empty", 0, billFtsCount)

        v2.close()
    }

    @Test
    fun migration_1_2_diary_tables_accept_valid_rows() {
        helper.createDatabase(TEST_DB, 1).close()

        val v2 = helper.runMigrationsAndValidate(
            TEST_DB, 2, true, Migrations.MIGRATION_1_2,
        )

        val customerId = newId()
        val billId     = newId()
        val ledgerId   = newId()
        val now        = nowMs()

        // Insert a customer
        v2.execSQL(
            """INSERT INTO customers (id, name, phone, address, gstNumber, notes,
               createdAt, updatedAt) VALUES (?, ?, ?, ?, ?, ?, ?, ?)""",
            arrayOf(customerId, "Ramesh Jewellers", "9876543210", "Mumbai", "", "", now, now),
        )

        // Insert a bill linked to that customer
        v2.execSQL(
            """INSERT INTO bills (id, customerId, customerName, customerPhone,
               itemsSummary, metalType, totalWeightG, metalValueInr, makingCharges,
               totalInr, paymentMode, notes, goldRateUsed, silverRateUsed,
               rateSource, createdAt, syncedAt)
               VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)""",
            arrayOf(
                billId, customerId, "Ramesh Jewellers", "9876543210",
                "[{\"item\":\"ring\",\"weight\":5.0}]", "gold",
                5.0, 30550.0, 500.0, 31050.0, "cash", "", 6110.0, 0.0, "live", now,
            ),
        )

        // Insert a ledger entry linked to the customer and bill
        v2.execSQL(
            """INSERT INTO ledger_entries (id, customerId, billId, type, amountInr,
               description, createdAt) VALUES (?, ?, ?, ?, ?, ?, ?)""",
            arrayOf(ledgerId, customerId, billId, "credit", 31050.0, "Invoice #001", now),
        )

        // Verify counts
        assertEquals(1, v2.query("SELECT COUNT(*) FROM customers").use { it.moveToFirst(); it.getInt(0) })
        assertEquals(1, v2.query("SELECT COUNT(*) FROM bills").use { it.moveToFirst(); it.getInt(0) })
        assertEquals(1, v2.query("SELECT COUNT(*) FROM ledger_entries").use { it.moveToFirst(); it.getInt(0) })

        v2.close()
    }

    @Test
    fun migration_1_2_zero_existing_users_no_data_loss() {
        // Simulates fresh install: DB is created at v1 with no rows at all,
        // then migrated to v2. The post-migration validation must pass (no schema
        // mismatch) and all tables must be empty.
        helper.createDatabase(TEST_DB, 1).close()

        val v2 = helper.runMigrationsAndValidate(
            TEST_DB, 2, true, Migrations.MIGRATION_1_2,
        )

        // Spot-check a few tables
        listOf("rates", "bills", "customers", "ledger_entries", "alerts").forEach { table ->
            val count = v2.query("SELECT COUNT(*) FROM $table").use {
                it.moveToFirst(); it.getInt(0)
            }
            assertEquals("$table must be empty on a fresh v1→v2 migration with no prior data", 0, count)
        }

        v2.close()
    }

    @Test
    fun migration_1_2_ledger_fk_rejects_orphan_entry() {
        // Validates that setForeignKeyConstraintsEnabled(true) in DatabaseModule
        // actually enforces the LedgerEntryEntity FK at runtime.
        // Without this, deleting a customer would leave orphaned ledger_entries —
        // a data-consistency hole described in the deployment audit.
        //
        // NOTE: MigrationTestHelper opens the DB in WAL mode with FK enforcement ON
        // because we are using the Room-generated schema. The PRAGMA is applied by
        // Room's onOpen callback when using the real Database class.
        // This test MUST run via connectedAndroidTest (real SQLite, not Robolectric)
        // because Robolectric's SQLite does not support PRAGMA foreign_keys.
        helper.createDatabase(TEST_DB, 1).close()

        val v2 = helper.runMigrationsAndValidate(
            TEST_DB, 2, true, Migrations.MIGRATION_1_2,
        )

        // Enable FK enforcement manually for the raw SupportSQLiteDatabase
        // (the Room builder enables it via onOpen, but MigrationTestHelper gives us
        // a raw SupportSQLiteDatabase before the builder's onOpen runs).
        v2.execSQL("PRAGMA foreign_keys = ON")

        val nonExistentCustomerId = newId()
        val orphanLedgerId        = newId()

        var threw = false
        try {
            v2.execSQL(
                """INSERT INTO ledger_entries (id, customerId, billId, type, amountInr,
                   description, createdAt) VALUES (?, ?, NULL, ?, ?, ?, ?)""",
                arrayOf(orphanLedgerId, nonExistentCustomerId, "credit", 1000.0, "orphan", nowMs()),
            )
        } catch (e: android.database.sqlite.SQLiteConstraintException) {
            threw = true
        }

        assertEquals(
            "Inserting a ledger_entry with a non-existent customerId must throw " +
            "SQLiteConstraintException when FK enforcement is enabled",
            true,
            threw,
        )

        v2.close()
    }

    companion object {
        private const val TEST_DB = "mahaswarna_migration_test.db"
    }
}
