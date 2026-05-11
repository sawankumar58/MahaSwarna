package com.mahaswarna.feature.rates.data.remote

import kotlinx.serialization.Serializable
import retrofit2.http.GET
import retrofit2.http.Path

// ── Wire DTOs ─────────────────────────────────────────────────────────────────

@Serializable
data class RateDto(
    val cityId: String,
    val gold: Double,
    val silver: Double,
    val source: String,         // not a closed enum — never hardcode "gemini"
    val generatedAt: String,    // ISO-8601 IST
    val stale: Boolean,         // always trust backend field — never compute client-side
)

@Serializable
data class RateHistoryPointDto(
    val gold: Double,
    val silver: Double,
    val source: String,
    val generatedAt: String,
    val stale: Boolean,
)

// ── Retrofit interface ────────────────────────────────────────────────────────

/**
 * REST fallback for rates (WS is the primary live source).
 * Both endpoints proxy through gateway (:4000) → pricing service (:4002).
 * Without the /history endpoint declaration, RateHistoryScreen has no data source (GAP-M4 fix).
 */
interface RatesApi {
    @GET("rates/{cityID}")
    suspend fun getRate(@Path("cityID") cityID: String): RateDto

    @GET("rates/{cityID}/history")
    suspend fun getRateHistory(@Path("cityID") cityID: String): List<RateHistoryPointDto>
}
