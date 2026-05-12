package com.mahaswarna.feature.rates.domain

import com.mahaswarna.feature.rates.data.RatesRepository
import javax.inject.Inject

/**
 * Fetches the current rate for a city via REST.
 *
 * Called by [RatesDashboardViewModel.fetchRate] when:
 *   - WebSocket is disconnected or unavailable.
 *   - User triggers a manual retry.
 *   - Kill-switch for WS is active (polling mode).
 *
 * Also updates [RatesRepository.currentRateFlow] as a side effect.
 */
class GetRateUseCase @Inject constructor(
    private val ratesRepository: RatesRepository,
) {
    suspend operator fun invoke(cityId: String): Rate = ratesRepository.getRate(cityId)
}
