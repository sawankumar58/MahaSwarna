package com.mahaswarna.feature.rates.data

import com.mahaswarna.feature.rates.data.remote.RateDto
import com.mahaswarna.feature.rates.data.remote.RateHistoryPointDto
import com.mahaswarna.feature.rates.data.remote.RatesApi
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Remote data source for rates — isolates network calls from [RatesRepository].
 *
 * All methods are suspend; callers handle errors via runCatching or try/catch.
 * Returns raw DTOs; mapping to domain models is the responsibility of [RatesRepository].
 */
@Singleton
class RatesRemoteDataSource @Inject constructor(
    private val ratesApi: RatesApi,
) {
    /** Fetches the current rate for a given city. Throws on network or HTTP error. */
    suspend fun getRate(cityId: String): RateDto = ratesApi.getRate(cityId)

    /** Fetches rate history for a given city. Throws on network or HTTP error. */
    suspend fun getRateHistory(cityId: String): List<RateHistoryPointDto> =
        ratesApi.getRateHistory(cityId)
}
