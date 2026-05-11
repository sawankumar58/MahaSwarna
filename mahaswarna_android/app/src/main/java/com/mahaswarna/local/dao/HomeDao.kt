package com.mahaswarna.local.dao

import androidx.room.Dao
import androidx.room.Query
import androidx.room.Upsert
import com.mahaswarna.local.entity.HomeEntity
import kotlinx.coroutines.flow.Flow

@Dao
interface HomeDao {
    @Upsert
    suspend fun upsert(home: HomeEntity)

    @Query("SELECT * FROM home WHERE id = 1 LIMIT 1")
    fun observe(): Flow<HomeEntity?>

    @Query("DELETE FROM home")
    suspend fun clearAll()
}
