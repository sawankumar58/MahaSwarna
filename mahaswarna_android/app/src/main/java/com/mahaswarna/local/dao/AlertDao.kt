package com.mahaswarna.local.dao

import androidx.room.Dao
import androidx.room.Query
import androidx.room.Upsert
import com.mahaswarna.local.entity.AlertEntity
import kotlinx.coroutines.flow.Flow

@Dao
interface AlertDao {
    @Upsert
    suspend fun upsertAll(alerts: List<AlertEntity>)

    @Upsert
    suspend fun upsert(alert: AlertEntity)

    @Query("SELECT * FROM alerts ORDER BY createdAt DESC")
    fun observeAll(): Flow<List<AlertEntity>>

    @Query("DELETE FROM alerts WHERE id = :id")
    suspend fun delete(id: String)

    @Query("DELETE FROM alerts")
    suspend fun clearAll()
}
