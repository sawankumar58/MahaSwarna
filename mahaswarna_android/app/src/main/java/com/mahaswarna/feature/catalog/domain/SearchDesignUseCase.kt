package com.mahaswarna.feature.catalog.domain

import com.mahaswarna.feature.catalog.data.CatalogRepository
import javax.inject.Inject

/**
 * Searches the catalog with a text query.
 * Hits the network with local FTS cache fallback via [CatalogRepository].
 *
 * Empty query → returns trending / all designs sorted by view count.
 */
class SearchDesignUseCase @Inject constructor(
    private val repository: CatalogRepository,
) {
    suspend operator fun invoke(
        query: String,
        region: String = "",
        metalType: String = "",
        page: Int = 1,
        pageSize: Int = 20,
    ): Result<DesignSearchResult> = runCatching {
        repository.search(
            query     = query.trim(),
            region    = region,
            metalType = metalType,
            page      = page,
            pageSize  = pageSize,
        )
    }
}
