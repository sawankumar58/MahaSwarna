package com.mahaswarna.feature.marketplace.domain

import com.mahaswarna.feature.marketplace.data.MarketplaceRepository
import com.mahaswarna.feature.marketplace.data.InvoiceRequest
import javax.inject.Inject

/**
 * Generates a PDF invoice via the backend intelligence service.
 *
 * ADR-001: PDF bytes are NOT stored server-side. The backend generates the PDF
 * in-memory, streams it to the client as application/pdf, and discards it.
 * Only invoice metadata (customer, items, amounts, rate snapshot) is persisted.
 *
 * Rate limits:
 *   - PREMIUM shops: 60 invoices per IST day (enforced via Redis counter server-side).
 *   - Backend returns HTTP 429 on [ErrDailyLimitExceeded]; mapped to [InvoiceResult.QuotaExceeded].
 *
 * On success the PDF bytes should be offered to the user via Android ShareSheet or
 * written to a local file — never uploaded back to the server.
 */
class GenerateInvoiceUseCase @Inject constructor(
    private val repository: MarketplaceRepository,
) {
    sealed class InvoiceResult {
        data class Success(val pdfBytes: ByteArray) : InvoiceResult()
        data object QuotaExceeded : InvoiceResult()
        data class Failure(val message: String) : InvoiceResult()
    }

    suspend operator fun invoke(request: InvoiceRequest): InvoiceResult {
        return runCatching {
            repository.generateInvoice(request)
        }.fold(
            onSuccess = { InvoiceResult.Success(it) },
            onFailure = { e ->
                if (e is retrofit2.HttpException && e.code() == 429) {
                    InvoiceResult.QuotaExceeded
                } else {
                    InvoiceResult.Failure(e.message ?: "Invoice generation failed")
                }
            }
        )
    }
}
