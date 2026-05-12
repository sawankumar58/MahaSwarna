package com.mahaswarna.feature.diary.domain

/**
 * Aggregated ledger summary for a customer, computed by [LedgerDao.getNetBalance].
 *
 * Convention:
 *   [netBalanceInr] > 0 → customer owes the shop (credit extended to customer).
 *   [netBalanceInr] < 0 → shop owes the customer (overpayment / advance).
 *   [netBalanceInr] == 0 → settled.
 */
data class LedgerSummary(
    val customerId: String,
    val customerName: String,
    /** Signed net balance: SUM(credits) - SUM(debits). */
    val netBalanceInr: Double,
    val totalTransactions: Int,
)
