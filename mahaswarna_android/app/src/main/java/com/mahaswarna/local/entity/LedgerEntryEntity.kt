package com.mahaswarna.local.entity

import androidx.room.Entity
import androidx.room.ForeignKey
import androidx.room.Index
import androidx.room.PrimaryKey

/**
 * Room entity for the `ledger_entries` table (Phase 2 Diary).
 *
 * Represents a single debit or credit entry in a customer's running account.
 * [billId] is null for manual entries not tied to a generated invoice.
 *
 * FK cascades on customer delete so the ledger stays consistent when
 * a customer is removed.
 */
@Entity(
    tableName = "ledger_entries",
    foreignKeys = [
        ForeignKey(
            entity        = CustomerEntity::class,
            parentColumns = ["id"],
            childColumns  = ["customerId"],
            onDelete      = ForeignKey.CASCADE,
        ),
    ],
    indices = [Index("customerId"), Index("createdAt")],
)
data class LedgerEntryEntity(
    @PrimaryKey val id: String,
    val customerId: String,
    /** Null for manual entries not linked to a bill. */
    val billId: String? = null,
    /** "credit" | "debit" */
    val type: String,
    val amountInr: Double,
    val description: String = "",
    /** Epoch milliseconds. */
    val createdAt: Long,
)
