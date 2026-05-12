package com.mahaswarna.feature.catalog.data

import kotlinx.serialization.Serializable
import okhttp3.MultipartBody
import retrofit2.http.GET
import retrofit2.http.Multipart
import retrofit2.http.Part
import retrofit2.http.Path
import retrofit2.http.Query

// ── Response DTOs ──────────────────────────────────────────────────────────────

@Serializable
data class DesignDto(
    val id: String,
    val title: String,
    val description: String = "",
    val category: String = "",
    val style: String = "",
    val region: String? = null,
    val metalType: String,
    val imageUrl: String,
    val tags: List<String> = emptyList(),
    val viewCount: Long = 0L,
    val shopId: String? = null,
    val weightGrams: Double = 0.0,
    val cdnKey: String = "",
)

@Serializable
data class DesignListDto(
    val designs: List<DesignDto>,
    val totalCount: Int = 0,
    val page: Int = 1,
    val totalPages: Int = 1,
)

// ── Retrofit interface ─────────────────────────────────────────────────────────

interface CatalogApi {

    /**
     * GET /v1/catalog/search?q=&region=&metal=&page=&limit=
     * Full-text search over the design catalog.
     * Empty [q] returns trending designs (ORDER BY view_count DESC).
     */
    @GET("catalog/search")
    suspend fun search(
        @Query("q")      query: String = "",
        @Query("region") region: String = "",
        @Query("metal")  metalType: String = "",
        @Query("page")   page: Int = 1,
        @Query("limit")  limit: Int = 20,
    ): DesignListDto

    /**
     * GET /v1/catalog/designs/{id}
     * Fetches a single design and records a view (view count buffered via Redis server-side).
     */
    @GET("catalog/designs/{id}")
    suspend fun getDesign(@Path("id") id: String): DesignDto

    /**
     * GET /v1/catalog/recommendations?region=&metal=&limit=
     * Trending recommendations for the catalog home section.
     */
    @GET("catalog/recommendations")
    suspend fun getRecommendations(
        @Query("region") region: String = "",
        @Query("metal")  metalType: String = "",
        @Query("limit")  limit: Int = 20,
    ): DesignListDto

    /**
     * POST /v1/catalog/image-search (multipart image upload).
     * GATED by killSwitchImageSearch — endpoint not enabled until kill-switch is false.
     * Returns visually similar designs from the catalog.
     */
    @Multipart
    @retrofit2.http.POST("catalog/image-search")
    suspend fun imageSearch(
        @Part image: MultipartBody.Part,
    ): DesignListDto
}
