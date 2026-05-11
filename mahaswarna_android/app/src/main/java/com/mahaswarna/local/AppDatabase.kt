package com.mahaswarna.local

import androidx.room.Database
import androidx.room.RoomDatabase
import androidx.room.migration.Migration
import androidx.sqlite.db.SupportSQLiteDatabase
import com.mahaswarna.local.dao.AlertDao
import com.mahaswarna.local.dao.DesignDao
import com.mahaswarna.local.dao.HomeDao
import com.mahaswarna.local.dao.RateDao
import com.mahaswarna.local.entity.AlertEntity
import com.mahaswarna.local.entity.DesignEntity
import com.mahaswarna.local.entity.HomeEntity
import com.mahaswarna.local.entity.RateEntity

// ─── MIGRATION POLICY — NON-NEGOTIABLE ────────────────────────────────────────
// NEVER call .fallbackToDestructiveMigration(). Diary tables (bills, ledger,
// customers) are local-only and unrecoverable — a destructive migration silently
// wipes a jeweller's entire transaction history with no recourse.
// Every schema version bump MUST have an explicit Migration below AND a
// @MigrationTest that asserts Diary row counts are identical before and after.
//
// Phase 2 migration template (add when Diary entities land):
//   val MIGRATION_1_2 = object : Migration(1, 2) {
//       override fun migrate(db: SupportSQLiteDatabase) {
//           db.execSQL("CREATE TABLE IF NOT EXISTS bills (...)")
//           db.execSQL("CREATE TABLE IF NOT EXISTS customers (...)")
//           db.execSQL("CREATE TABLE IF NOT EXISTS ledger_entries (...)")
//       }
//   }
// ──────────────────────────────────────────────────────────────────────────────

// Phase 1 entities — session-scoped (cleared on logout).
// Phase 2 will add: BillEntity, BillFts, CustomerEntity, CustomerFts,
//   LedgerEntryEntity — bump version to 2 with explicit Migration.
@Database(
    entities = [
        RateEntity::class,
        HomeEntity::class,
        AlertEntity::class,
        DesignEntity::class,
    ],
    version = 1,
    exportSchema = true,
)
abstract class AppDatabase : RoomDatabase() {

    abstract fun rateDao(): RateDao
    abstract fun homeDao(): HomeDao
    abstract fun alertDao(): AlertDao
    abstract fun designDao(): DesignDao

    // billDao() / customerDao() / ledgerDao() added in Phase 2.

    /**
     * Clears ONLY session-scoped tables (rates, home, alerts, designs).
     * MUST NOT touch Diary tables (bill, customer, ledger) — they are
     * local-only and unrecoverable. Called on logout and token expiry.
     */
    suspend fun clearSessionData() {
        rateDao().clearAll()
        homeDao().clearAll()
        alertDao().clearAll()
        designDao().clearAll()
    }

    /**
     * Full wipe of ALL tables including Diary (Phase 2+).
     * Called ONLY from DeleteAccountUseCase after the server confirms 204
     * on DELETE /user/account.
     */
    suspend fun clearAll() {
        clearAllTables()   // Room built-in — clears every registered entity table
    }
}
