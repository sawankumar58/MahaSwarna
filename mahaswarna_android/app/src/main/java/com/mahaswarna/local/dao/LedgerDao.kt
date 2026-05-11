package com.mahaswarna.local.dao

import androidx.room.Dao
import androidx.room.Query
import androidx.room.Upsert
import com.mahaswarna.local.entity.LedgerEntryEntity
import kotlinx.coroutines.flow.Flow

@Dao
interface LedgerDao {

    @Upsert
    suspend fun upsert(entry: LedgerEntryEntity)

    @Upsert
    suspend fun upsertAll(entries: List<LedgerEntryEntity>)

    @Query("SELECT * FROM ledger_entries WHERE customerId = :customerId ORDER BY createdAt DESC")
    fun observeByCustomer(customerId: String): Flow<List<LedgerEntryEntity>>

    /**
     * Returns the net balance for a customer: SUM(credits) - SUM(debits).
     * Positive = customer owes the shop; negative = shop owes the customer.
     */
    @Query("""
        SELECT COALESCE(
            SUM(CASE WHEN type = 'credit' THEN amountInr ELSE -amountInr END), 0.0
        ) FROM ledger_entries WHERE customerId = :customerId
    """)
    suspend fun getNetBalance(customerId: String): Double

    @Query("DELETE FROM ledger_entries WHERE id = :id")
    suspend fun delete(id: String)

    @Query("DELETE FROM ledger_entries WHERE customerId = :customerId")
    suspend fun deleteByCustomer(customerId: String)

    @Query("DELETE FROM ledger_entries")
    suspend fun clearAll()
}
