package com.mahaswarna.local.dao

import androidx.room.Dao
import androidx.room.Query
import androidx.room.Upsert
import com.mahaswarna.local.entity.DesignEntity
import kotlinx.coroutines.flow.Flow

@Dao
interface DesignDao {
    @Upsert
    suspend fun upsertAll(designs: List<DesignEntity>)

    @Query("SELECT * FROM designs ORDER BY title ASC")
    fun observeAll(): Flow<List<DesignEntity>>

    @Query("SELECT * FROM designs WHERE id = :id LIMIT 1")
    suspend fun getById(id: String): DesignEntity?

    @Query("DELETE FROM designs")
    suspend fun clearAll()
}
