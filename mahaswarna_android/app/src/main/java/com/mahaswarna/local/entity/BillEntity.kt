package com.mahaswarna.local.entity

import androidx.room.Entity
import androidx.room.Fts4
import androidx.room.FtsOptions
import androidx.room.PrimaryKey

/**
 * Room entity for the `bills` table (Phase 2 Diary).
 *
 * INVARIANT: Diary data is local-only and unrecoverable.
 * [syncedAt] is reserved for future server backup — currently always null.
 * [goldRateUsed] / [silverRateUsed] capture the live rate at time of invoice generation.
 */
@Entity(tableName = "bills")
data class BillEntity(
    @PrimaryKey val id: String,
    val customerId: String = "",
    val customerName: String,
    val customerPhone: String = "",
    /** JSON array of InvoiceLineItem serialised as a String. */
    val itemsSummary: String,
    /** "gold" | "silver" | "mixed" */
    val metalType: String,
    val totalWeightG: Double,
    val metalValueInr: Double,
    val makingCharges: Double = 0.0,
    val totalInr: Double,
    /** "cash" | "upi" | "cheque" | "card" */
    val paymentMode: String = "cash",
    val notes: String = "",
    /** Per-gram gold rate at the moment of bill creation. */
    val goldRateUsed: Double,
    /** Per-gram silver rate at the moment of bill creation (0.0 for gold-only bills). */
    val silverRateUsed: Double = 0.0,
    /** "live" | "manual" — whether the rate was from the live feed or entered manually. */
    val rateSource: String = "live",
    /** Epoch milliseconds (local device time). */
    val createdAt: Long,
    /** Epoch ms when last synced to server, or null if never synced. */
    val syncedAt: Long? = null,
)

/**
 * FTS4 virtual table backed by [BillEntity].
 * Room keeps this index in sync automatically via content-based FTS.
 * Enables full-text search on customer name and items summary.
 */
@Fts4(contentEntity = BillEntity::class)
@Entity(tableName = "bill_fts")
data class BillFts(
    val customerName: String,
    val itemsSummary: String,
)
