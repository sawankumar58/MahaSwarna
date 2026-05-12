package com.mahaswarna.feature.catalog.data

import com.mahaswarna.feature.catalog.domain.Design
import com.mahaswarna.feature.catalog.domain.DesignSearchResult
import com.mahaswarna.local.dao.DesignDao
import com.mahaswarna.local.entity.DesignEntity
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import okhttp3.MediaType.Companion.toMediaTypeOrNull
import okhttp3.MultipartBody
import okhttp3.RequestBody.Companion.toRequestBody
import javax.inject.Inject
import javax.inject.Singleton

/**
 * CatalogRepository — network-first with local [DesignDao] cache fallback.
 *
 * Cache strategy:
 *   - Network success → upsert page-1 results to DesignDao (session-scoped; cleared on logout).
 *   - Network failure → serve from Room cache if available (page 1 only).
 *   - Image search is NOT cached (results are query-specific and stateless).
 *
 * FIX: Previous version defined a private `first(Flow<T>)` extension that called
 * itself recursively (infinite loop). Replaced with the standard `kotlinx.coroutines.flow.first`
 * imported at the top of the file.
 */
@Singleton
class CatalogRepository @Inject constructor(
    private val api: CatalogApi,
    private val designDao: DesignDao,
) {

    /** Network-first paginated search. Falls back to Room cache on page-1 failure. */
    suspend fun search(
        query: String,
        region: String,
        metalType: String,
        page: Int,
        pageSize: Int,
    ): DesignSearchResult {
        return runCatching {
            val dto = api.search(
                query     = query,
                region    = region,
                metalType = metalType,
                page      = page,
                limit     = pageSize,
            )
            if (page == 1) designDao.upsertAll(dto.designs.map { it.toEntity() })
            dto.toDomain()
        }.getOrElse { networkError ->
            if (page != 1) throw networkError          // only fall back for page-1
            // FIX: use imported kotlinx.coroutines.flow.first — not a self-recursive helper
            val cached = designDao.observeAll().first()
            val filtered = cached.filter { entity ->
                (metalType.isBlank() || entity.metal == metalType) &&
                (query.isBlank() || entity.title.contains(query, ignoreCase = true))
            }
            DesignSearchResult(
                designs    = filtered.map { it.toDomain() },
                totalCount = filtered.size,
                page       = 1,
                totalPages = 1,
            )
        }
    }

    /** Fetches a single design; server-side increments view_count via Redis. */
    suspend fun getDesign(id: String): Design = api.getDesign(id).toDomain()

    /** Trending designs — network-only; cached into Room by [search] page-1 on the caller. */
    suspend fun getRecommendations(
        region: String = "",
        metalType: String = "",
        limit: Int = 20,
    ): List<Design> = api.getRecommendations(region, metalType, limit).designs.map { it.toDomain() }

    /** Visual image search — NOT cached. Requires killSwitchImageSearch == false. */
    suspend fun imageSearch(imageBytes: ByteArray): List<Design> {
        val body = imageBytes.toRequestBody("image/*".toMediaTypeOrNull())
        val part = MultipartBody.Part.createFormData("image", "query.jpg", body)
        return api.imageSearch(part).designs.map { it.toDomain() }
    }

    /** Observes the local DesignDao cache as a Flow for the offline degraded state. */
    fun observeCache(): Flow<List<Design>> =
        designDao.observeAll().map { list -> list.map { it.toDomain() } }

    // ── Mappers ────────────────────────────────────────────────────────────────

    private fun DesignDto.toDomain() = Design(
        id          = id,
        title       = title,
        description = description,
        category    = category,
        style       = style,
        region      = region,
        metalType   = metalType,
        imageUrl    = imageUrl,
        tags        = tags,
        viewCount   = viewCount,
        shopId      = shopId,
        weightGrams = weightGrams,
        cdnKey      = cdnKey,
    )

    private fun DesignListDto.toDomain() = DesignSearchResult(
        designs    = designs.map { it.toDomain() },
        totalCount = totalCount,
        page       = page,
        totalPages = totalPages,
    )

    private fun DesignDto.toEntity() = DesignEntity(
        id          = id,
        title       = title,
        metal       = metalType,
        weightGrams = weightGrams,
        imageUrl    = imageUrl,
        cdnKey      = cdnKey,
        cachedAt    = System.currentTimeMillis(),
    )

    private fun DesignEntity.toDomain() = Design(
        id          = id,
        title       = title,
        metalType   = metal,
        imageUrl    = imageUrl,
        weightGrams = weightGrams,
        cdnKey      = cdnKey,
    )
}
