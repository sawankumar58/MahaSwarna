package com.mahaswarna.feature.diary.domain

import kotlinx.serialization.Serializable

/**
 * Domain model for a local invoice (bill) generated on-device.
 *
 * INVARIANT: Diary data is local-only and unrecoverable.
 * [syncedAt] is null until a future backup feature is implemented.
 * [goldRateUsed] / [silverRateUsed] capture the live rate snapshot at the time the
 * bill was created — never re-compute from current rates.
 */
data class DiaryBill(
    val id: String,
    val customerId: String = "",
    val customerName: String,
    val customerPhone: String = "",
    /** JSON-serialised list of [InvoiceLineItem]. */
    val itemsSummary: String,
    /** Parsed list of line items (lazy from [itemsSummary]). */
    val lineItems: List<InvoiceLineItem> = emptyList(),
    /** "gold" | "silver" | "mixed" */
    val metalType: String,
    val totalWeightG: Double,
    val metalValueInr: Double,
    val makingCharges: Double = 0.0,
    val totalInr: Double,
    /** "cash" | "upi" | "cheque" | "card" */
    val paymentMode: String = "cash",
    val notes: String = "",
    val goldRateUsed: Double,
    val silverRateUsed: Double = 0.0,
    /** "live" | "manual" */
    val rateSource: String = "live",
    /** Epoch milliseconds (device local time). */
    val createdAt: Long,
    val syncedAt: Long? = null,
)

/** A single line item in an invoice — JSON structure inside [DiaryBill.itemsSummary]. */
@Serializable
data class InvoiceLineItem(
    val description: String,
    val weightGrams: Double,
    val karat: Int = 22,
    val makingCharge: Double = 0.0,
    val unitPrice: Double,
    val totalPrice: Double,
    val gstAmount: Double = 0.0,
    val netAmount: Double,
)
