package com.mahaswarna.feature.home.domain

import com.mahaswarna.feature.alerts.domain.Alert

/**
 * Aggregated home screen data.
 * Composed from BFF /bff/home response and persisted to Room.
 * WS push updates replace `rate` in-memory via HomeRepository.applyWsRateUpdate().
 */
data class HomeData(
    val rate: RateInfo,
    val alerts: List<Alert> = emptyList(),
)

/**
 * Rate information for a single city.
 *
 * isStale: always sourced from backend `stale` field — NEVER computed client-side from cachedAt.
 * source:  not a closed enum — pass through verbatim to analytics.
 */
data class RateInfo(
    val cityId: String,
    val gold: Double,
    val silver: Double,
    val source: String,      // "gemini" | future values — never hardcode "gemini" as a literal
    val generatedAt: String, // ISO-8601 IST
    val isStale: Boolean,    // from backend field — never derive client-side
)
