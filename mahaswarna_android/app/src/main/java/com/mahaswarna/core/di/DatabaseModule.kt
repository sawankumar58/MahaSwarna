package com.mahaswarna.core.di

import android.content.Context
import androidx.room.Room
import com.mahaswarna.local.AppDatabase
import com.mahaswarna.local.dao.AlertDao
import com.mahaswarna.local.dao.BillDao
import com.mahaswarna.local.dao.CustomerDao
import com.mahaswarna.local.dao.DesignDao
import com.mahaswarna.local.dao.HomeDao
import com.mahaswarna.local.dao.LedgerDao
import com.mahaswarna.local.dao.RateDao
import com.mahaswarna.local.migration.Migrations
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.android.qualifiers.ApplicationContext
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
object DatabaseModule {

    @Provides
    @Singleton
    fun provideAppDatabase(@ApplicationContext context: Context): AppDatabase =
        Room.databaseBuilder(context, AppDatabase::class.java, "mahaswarna.db")
            // NEVER .fallbackToDestructiveMigration()
            // Diary data is local-only and unrecoverable — see Migrations.kt.
            .addMigrations(Migrations.MIGRATION_1_2)
            .build()

    // ── Session-scoped DAOs ───────────────────────────────────────────────

    @Provides fun provideRateDao(db: AppDatabase): RateDao = db.rateDao()
    @Provides fun provideHomeDao(db: AppDatabase): HomeDao = db.homeDao()
    @Provides fun provideAlertDao(db: AppDatabase): AlertDao = db.alertDao()
    @Provides fun provideDesignDao(db: AppDatabase): DesignDao = db.designDao()

    // ── Phase 2 Diary DAOs ────────────────────────────────────────────────

    @Provides fun provideBillDao(db: AppDatabase): BillDao = db.billDao()
    @Provides fun provideCustomerDao(db: AppDatabase): CustomerDao = db.customerDao()
    @Provides fun provideLedgerDao(db: AppDatabase): LedgerDao = db.ledgerDao()
}
