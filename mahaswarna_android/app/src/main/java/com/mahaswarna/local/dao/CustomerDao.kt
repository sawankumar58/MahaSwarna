package com.mahaswarna.local.dao

import androidx.room.Dao
import androidx.room.Query
import androidx.room.Upsert
import com.mahaswarna.local.entity.CustomerEntity
import kotlinx.coroutines.flow.Flow

@Dao
interface CustomerDao {

    @Upsert
    suspend fun upsert(customer: CustomerEntity)

    @Upsert
    suspend fun upsertAll(customers: List<CustomerEntity>)

    @Query("SELECT * FROM customers ORDER BY name ASC")
    fun observeAll(): Flow<List<CustomerEntity>>

    @Query("SELECT * FROM customers WHERE id = :id")
    suspend fun getById(id: String): CustomerEntity?

    @Query("DELETE FROM customers WHERE id = :id")
    suspend fun delete(id: String)

    /**
     * FTS search on customer name.
     * [query] must be a valid FTS match expression (e.g. "sharma*").
     */
    @Query("""
        SELECT customers.* FROM customers
        INNER JOIN customer_fts ON customers.rowid = customer_fts.rowid
        WHERE customer_fts MATCH :query
        ORDER BY customers.name ASC
    """)
    fun search(query: String): Flow<List<CustomerEntity>>

    @Query("DELETE FROM customers")
    suspend fun clearAll()
}
