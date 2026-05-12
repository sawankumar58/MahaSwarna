package com.mahaswarna.feature.diary.domain

import com.mahaswarna.feature.diary.data.DiaryRepository
import kotlinx.coroutines.flow.Flow
import javax.inject.Inject

/**
 * Observes all ledger entries for a given customer, newest first.
 * Backed by [DiaryRepository.observeLedgerByCustomer].
 */
class GetCustomerLedgerUseCase @Inject constructor(
    private val repository: DiaryRepository,
) {
    /** Returns a [Flow] of [LedgerEntry] list that updates on every DB write. */
    operator fun invoke(customerId: String): Flow<List<LedgerEntry>> =
        repository.observeLedgerByCustomer(customerId)
}
