package com.mahaswarna.local.dao

import androidx.room.Dao
import androidx.room.Query
import androidx.room.Upsert
import com.mahaswarna.local.entity.RateEntity
import kotlinx.coroutines.flow.Flow

@Dao
interface RateDao {
    @Upsert
    suspend fun upsertRate(rate: RateEntity)

    @Query("SELECT * FROM rates WHERE cityId = :cityId LIMIT 1")
    fun getLatest(cityId: String): Flow<RateEntity?>

    /** Full history for the Vico chart — populated by REST, not WS. */
    @Query("SELECT * FROM rates WHERE cityId = :cityId ORDER BY generatedAt DESC")
    fun getHistory(cityId: String): Flow<List<RateEntity>>

    @Query("DELETE FROM rates")
    suspend fun clearAll()
}
