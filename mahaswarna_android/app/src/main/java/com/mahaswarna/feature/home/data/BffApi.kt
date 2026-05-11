package com.mahaswarna.feature.home.data

import com.mahaswarna.feature.alerts.data.AlertsApi  // AlertResponse reused
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import retrofit2.http.GET

// ── Wire DTOs ─────────────────────────────────────────────────────────────────

/**
 * BFF home aggregation response.
 *
 * _degraded: set to true by home_aggregator.go when any upstream (pricing or core/alerts)
 * times out and stale cache is served. Transient signal — NOT persisted to Room.
 * Map: @SerialName("_degraded") → Kotlin property `degraded`.
 * When true, HomeViewModel MUST show StaleRateBanner immediately.
 * Clears when the next response arrives with degraded == false (or field absent).
 */
@Serializable
data class HomeResponse(
    val rates: RateDto,
    val alerts: List<AlertResponseDto>? = null,
    @SerialName("_degraded") val degraded: Boolean = false,
)

@Serializable
data class RateDto(
    val cityId: String,
    val gold: Double,
    val silver: Double,
    val source: String,         // NOT a closed enum — new values may be added server-side
    val generatedAt: String,    // ISO-8601 IST timestamp
    val stale: Boolean,         // NEVER compute staleness client-side — always trust this field
)

@Serializable
data class AlertResponseDto(
    val id: String,
    val metal: String,
    val targetPrice: Double,
    val direction: String,
    val active: Boolean,
)

// ── Retrofit interface ────────────────────────────────────────────────────────

interface BffApi {
    /**
     * GET /bff/home — aggregated home data (rates + alerts).
     * Requires Bearer token (AuthInterceptor attaches automatically).
     * Called on every app resume; also polled every 30s ±5s when killSwitchWs == true.
     */
    @GET("bff/home")
    suspend fun getHome(): HomeResponse
}
