package com.mahaswarna.feature.rates.domain

/**
 * Domain rate model.
 *
 * isStale: ALWAYS sourced from backend `stale` field.
 * NEVER compute from cachedAt timestamp — the backend's Gemini generation window
 * is the authoritative staleness signal; client timestamps are unreliable.
 *
 * source: not a closed enum. Pass through verbatim to analytics `rate_viewed { source }`.
 * Never hardcode "gemini" as a string literal at the call site.
 */
data class Rate(
    val cityId: String,
    val gold: Double,
    val silver: Double,
    val source: String,       // e.g. "gemini"; future: "mcx", "manual_override"
    val generatedAt: String,  // ISO-8601 IST
    val isStale: Boolean,
)

/**
 * Single point in a city's rate history.
 * Used by RateHistoryScreen / RateHistoryViewModel.
 * No Room cache for history — network-required.
 */
data class RateHistoryPoint(
    val gold: Double,
    val silver: Double,
    val source: String,
    val generatedAt: String,  // ISO-8601 IST — parsed to LocalDateTime for chart X-axis
    val isStale: Boolean,
)
