package com.mahaswarna.feature.diary.domain

import com.mahaswarna.feature.diary.data.DiaryRepository
import java.util.UUID
import javax.inject.Inject

/**
 * Adds a manual ledger entry (credit or debit) for a customer.
 * Generates a new UUID client-side as the entry ID.
 */
class AddLedgerEntryUseCase @Inject constructor(
    private val repository: DiaryRepository,
) {
    data class Input(
        val customerId: String,
        val type: String,           // "credit" | "debit"
        val amountInr: Double,
        val description: String = "",
        val billId: String? = null,
    )

    suspend operator fun invoke(input: Input) {
        require(input.type == "credit" || input.type == "debit") {
            "type must be 'credit' or 'debit', got '${input.type}'"
        }
        require(input.amountInr > 0) { "amountInr must be positive" }

        val entry = LedgerEntry(
            id          = UUID.randomUUID().toString(),
            customerId  = input.customerId,
            billId      = input.billId,
            type        = input.type,
            amountInr   = input.amountInr,
            description = input.description,
            createdAt   = System.currentTimeMillis(),
        )
        repository.upsertLedgerEntry(entry)
    }
}
