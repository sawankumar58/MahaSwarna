package com.mahaswarna.local

import androidx.room.Database
import androidx.room.RoomDatabase
import com.mahaswarna.local.dao.AlertDao
import com.mahaswarna.local.dao.DesignDao
import com.mahaswarna.local.dao.HomeDao
import com.mahaswarna.local.dao.RateDao
import com.mahaswarna.local.dao.BillDao
import com.mahaswarna.local.dao.CustomerDao
import com.mahaswarna.local.dao.LedgerDao
import com.mahaswarna.local.entity.AlertEntity
import com.mahaswarna.local.entity.DesignEntity
import com.mahaswarna.local.entity.HomeEntity
import com.mahaswarna.local.entity.RateEntity
import com.mahaswarna.local.entity.BillEntity
import com.mahaswarna.local.entity.CustomerEntity
import com.mahaswarna.local.entity.LedgerEntryEntity

// ─── MIGRATION POLICY — NON-NEGOTIABLE ────────────────────────────────────────
// NEVER call .fallbackToDestructiveMigration(). Diary tables (bills, ledger,
// customers) are local-only and unrecoverable — a destructive migration silently
// wipes a jeweller's entire transaction history with no recourse.
// Every schema version bump MUST have an explicit Migration in Migrations.kt AND
// a @MigrationTest asserting that Diary row counts are identical before and after.
// ──────────────────────────────────────────────────────────────────────────────

@Database(
    entities = [
        // ── Phase 1 — session-scoped (cleared on logout) ──────────────────
        RateEntity::class,
        HomeEntity::class,
        AlertEntity::class,
        DesignEntity::class,
        // ── Phase 2 — Diary (local-only, NEVER cleared on logout) ─────────
        BillEntity::class,
        CustomerEntity::class,
        LedgerEntryEntity::class,
    ],
    version = 2,
    exportSchema = true,
)
abstract class AppDatabase : RoomDatabase() {

    // ── Phase 1 DAOs ──────────────────────────────────────────────────────
    abstract fun rateDao(): RateDao
    abstract fun homeDao(): HomeDao
    abstract fun alertDao(): AlertDao
    abstract fun designDao(): DesignDao

    // ── Phase 2 Diary DAOs ────────────────────────────────────────────────
    abstract fun billDao(): BillDao
    abstract fun customerDao(): CustomerDao
    abstract fun ledgerDao(): LedgerDao

    /**
     * Clears ONLY session-scoped tables (rates, home, alerts, designs).
     * MUST NOT touch Diary tables (bills, customers, ledger_entries) — they are
     * local-only and unrecoverable. Called on logout and token expiry.
     */
    suspend fun clearSessionData() {
        rateDao().clearAll()
        homeDao().clearAll()
        alertDao().clearAll()
        designDao().clearAll()
    }

    /**
     * Full wipe of ALL tables including Diary.
     * Called ONLY from DeleteAccountUseCase after the server confirms 204
     * on DELETE /user/account. Diary data is destroyed; this is intentional and
     * the user must confirm before this path is reached.
     */
    suspend fun clearAll() {
        clearAllTables() // Room built-in — clears every registered entity table
    }
}
