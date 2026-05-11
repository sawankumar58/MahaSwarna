package com.mahaswarna.core.di

import android.content.Context
import androidx.room.Room
import com.mahaswarna.local.AppDatabase
import com.mahaswarna.local.dao.AlertDao
import com.mahaswarna.local.dao.DesignDao
import com.mahaswarna.local.dao.HomeDao
import com.mahaswarna.local.dao.RateDao
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
            // Add explicit migrations here when the schema version is bumped:
            // .addMigrations(MIGRATION_1_2)
            .build()

    // ── Session-scoped DAOs ───────────────────────────────────────────────────

    @Provides fun provideRateDao(db: AppDatabase): RateDao     = db.rateDao()
    @Provides fun provideHomeDao(db: AppDatabase): HomeDao     = db.homeDao()
    @Provides fun provideAlertDao(db: AppDatabase): AlertDao   = db.alertDao()
    @Provides fun provideDesignDao(db: AppDatabase): DesignDao = db.designDao()

    // ── Diary DAOs — added in Phase 2 (feature/diary/data/local/) ────────────
    // DatabaseModule.provideBillDao / provideCustomerDao / provideLedgerDao
    // will be provided by feature/diary/di/DiaryModule.kt in Phase 2.
    // DO NOT add them here until the Diary entities compile.
}
