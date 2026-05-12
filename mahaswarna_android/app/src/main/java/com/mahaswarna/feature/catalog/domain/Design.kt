package com.mahaswarna.feature.catalog.domain

/**
 * Domain model for a jewellery design from the catalog.
 *
 * Derived from [com.mahaswarna.local.entity.DesignEntity] (offline cache) or
 * the network DTO from [CatalogApi].
 *
 * [region] is null for designs applicable to all regions.
 * [viewCount] is maintained server-side via Redis INCR; treat as eventually consistent.
 */
data class Design(
    val id: String,
    val title: String,
    val description: String = "",
    val category: String = "",
    val style: String = "",
    /** null = all regions applicable */
    val region: String? = null,
    /** "gold" | "silver" | "both" */
    val metalType: String,
    val imageUrl: String,
    val tags: List<String> = emptyList(),
    val viewCount: Long = 0L,
    val shopId: String? = null,
    val weightGrams: Double = 0.0,
    val cdnKey: String = "",
)

/** Paginated result from the catalog search endpoint. */
data class DesignSearchResult(
    val designs: List<Design>,
    val totalCount: Int,
    val page: Int,
    val totalPages: Int,
)
