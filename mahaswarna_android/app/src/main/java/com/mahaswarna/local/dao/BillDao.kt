package com.mahaswarna.local.dao

import androidx.room.Dao
import androidx.room.Query
import androidx.room.Upsert
import com.mahaswarna.local.entity.BillEntity
import kotlinx.coroutines.flow.Flow

@Dao
interface BillDao {

    @Upsert
    suspend fun upsert(bill: BillEntity)

    @Upsert
    suspend fun upsertAll(bills: List<BillEntity>)

    @Query("SELECT * FROM bills ORDER BY createdAt DESC")
    fun observeAll(): Flow<List<BillEntity>>

    @Query("SELECT * FROM bills WHERE customerId = :customerId ORDER BY createdAt DESC")
    fun observeByCustomer(customerId: String): Flow<List<BillEntity>>

    @Query("SELECT * FROM bills WHERE id = :id")
    suspend fun getById(id: String): BillEntity?

    @Query("DELETE FROM bills WHERE id = :id")
    suspend fun delete(id: String)

    /**
     * FTS search across customerName and itemsSummary.
     * [query] must be a valid FTS match expression (e.g. "ring*").
     */
    @Query("""
        SELECT bills.* FROM bills
        INNER JOIN bill_fts ON bills.rowid = bill_fts.rowid
        WHERE bill_fts MATCH :query
        ORDER BY bills.createdAt DESC
    """)
    fun search(query: String): Flow<List<BillEntity>>

    @Query("DELETE FROM bills")
    suspend fun clearAll()
}
