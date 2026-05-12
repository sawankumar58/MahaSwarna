package com.mahaswarna.feature.home.domain

import com.mahaswarna.feature.home.data.HomeRepository
import kotlinx.coroutines.flow.Flow
import javax.inject.Inject

/**
 * Observes cached home data from Room and triggers a REST refresh.
 *
 * Two responsibilities:
 *   1. [observeCached] — returns a [Flow<HomeData?>] backed by Room; emits on every
 *      cache update without hitting the network.
 *   2. [refresh] — triggers a BFF REST fetch and persists the result to Room,
 *      which causes [observeCached] to emit the updated data automatically.
 *
 * [HomeViewModel] uses both: observeCached drives the UI, refresh is called on
 * init and on retry. cityId is sourced from [PreferenceStore.getSelectedCityId].
 */
class GetHomeDataUseCase @Inject constructor(
    private val homeRepository: HomeRepository,
) {
    fun observeCached(): Flow<HomeData?> = homeRepository.getCachedHome()

    suspend fun refresh() = homeRepository.refresh()
}
