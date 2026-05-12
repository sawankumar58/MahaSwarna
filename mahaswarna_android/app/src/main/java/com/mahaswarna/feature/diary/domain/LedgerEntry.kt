package com.mahaswarna.feature.diary.domain

/**
 * Domain model for a single ledger entry (debit or credit) in a customer's account.
 *
 * [billId] is null for manual entries not linked to a generated invoice.
 * Derived from [com.mahaswarna.local.entity.LedgerEntryEntity] via mapper.
 */
data class LedgerEntry(
    val id: String,
    val customerId: String,
    /** Null for manual entries not tied to a bill. */
    val billId: String? = null,
    /** "credit" — customer paid; "debit" — customer owes. */
    val type: String,
    val amountInr: Double,
    val description: String = "",
    /** Epoch milliseconds. */
    val createdAt: Long,
)
