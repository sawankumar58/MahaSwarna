package com.mahaswarna.feature.catalog.domain

import com.mahaswarna.feature.catalog.data.CatalogRepository
import com.mahaswarna.feature.flags.data.FlagsRepository
import javax.inject.Inject

/**
 * Image-based visual design search.
 *
 * GATE: Callers MUST check [FlagsRepository.getFlags().killSwitchImageSearch] == false before
 * invoking. This use case also performs an internal guard and returns [Result.failure]
 * with [ImageSearchDisabledException] when the kill-switch is active.
 *
 * Route.ImageSearch must NOT be present in NavHost while [killSwitchImageSearch] == true.
 * See Route.kt comment and AppNavGraph.kt for the gating convention.
 */
class ImageSearchUseCase @Inject constructor(
    private val repository: CatalogRepository,
    private val flagsRepository: FlagsRepository,
) {
    class ImageSearchDisabledException : Exception("Image search is currently disabled")

    suspend operator fun invoke(imageBytes: ByteArray): Result<List<Design>> = runCatching {
        if (flagsRepository.getFlags().killSwitchImageSearch) {
            throw ImageSearchDisabledException()
        }
        repository.imageSearch(imageBytes)
    }
}
