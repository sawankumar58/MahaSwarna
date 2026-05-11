package com.mahaswarna.local.entity

import androidx.room.Entity
import androidx.room.Fts4
import androidx.room.PrimaryKey

/**
 * Room entity for the `customers` table (Phase 2 Diary).
 *
 * Represents an entry in the jeweller's local address book.
 * [gstNumber] is optional and stored as an empty string when absent.
 */
@Entity(tableName = "customers")
data class CustomerEntity(
    @PrimaryKey val id: String,
    val name: String,
    val phone: String = "",
    val address: String = "",
    val gstNumber: String = "",
    val notes: String = "",
    /** Epoch milliseconds. */
    val createdAt: Long,
    val updatedAt: Long,
)

/**
 * FTS4 virtual table backed by [CustomerEntity].
 * Enables full-text search on customer name from the Diary customers tab.
 */
@Fts4(contentEntity = CustomerEntity::class)
@Entity(tableName = "customer_fts")
data class CustomerFts(
    val name: String,
)
