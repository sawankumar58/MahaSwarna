package com.mahaswarna.data.mapper

import com.mahaswarna.feature.alerts.domain.Alert
import com.mahaswarna.feature.home.data.AlertResponseDto
import com.mahaswarna.feature.home.data.HomeResponse
import com.mahaswarna.feature.home.data.RateDto
import com.mahaswarna.feature.home.domain.HomeData
import com.mahaswarna.feature.home.domain.RateInfo
import com.mahaswarna.local.entity.AlertEntity
import com.mahaswarna.local.entity.HomeEntity
import com.mahaswarna.local.entity.RateEntity
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

// ── Private JSON instance ──────────────────────────────────────────────────
// Lenient so new fields added server-side don't crash existing clients.
private val json = Json { ignoreUnknownKeys = true; coerceInputValues = true }

// ═══════════════════════════════════════════════════════════════════════════
// RateDto ↔ RateEntity ↔ RateInfo
// ═══════════════════════════════════════════════════════════════════════════

/** Network DTO → Room entity for persistence. */
fun RateDto.toEntity(cachedAt: Long = System.currentTimeMillis()): RateEntity = RateEntity(
    cityId      = cityId,
    gold        = gold,
    silver      = silver,
    source      = source,
    generatedAt = generatedAt,
    isStale     = stale,
    cachedAt    = cachedAt,
)

/** Room entity → domain model for UI. */
fun RateEntity.toDomain(): RateInfo = RateInfo(
    cityId      = cityId,
    gold        = gold,
    silver      = silver,
    source      = source,
    generatedAt = generatedAt,
    isStale     = isStale,
)

/** Network DTO → domain model (in-memory path, no Room hop needed). */
fun RateDto.toDomain(): RateInfo = RateInfo(
    cityId      = cityId,
    gold        = gold,
    silver      = silver,
    source      = source,
    generatedAt = generatedAt,
    isStale     = stale,
)

// ═══════════════════════════════════════════════════════════════════════════
// AlertResponseDto ↔ AlertEntity ↔ Alert (domain)
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Network DTO → Room entity.
 * AlertResponseDto from BFF does NOT include cityId or threshold — those fields
 * come from the full /alerts endpoint. For BFF-sourced alerts use [AlertResponseDto.toDomain]
 * and store the JSON blob in HomeEntity.alertsJson.
 */
fun AlertResponseDto.toEntity(cityId: String, threshold: Float, createdAt: String): AlertEntity =
    AlertEntity(
        id        = id,
        cityId    = cityId,
        metal     = metal,
        direction = direction,
        threshold = threshold,
        active    = active,
        createdAt = createdAt,
    )

/** Room entity → domain model. */
fun AlertEntity.toDomain(): Alert = Alert(
    id          = id,
    metal       = metal,
    targetPrice = threshold.toDouble(),
    direction   = direction,
    active      = active,
)

/** Network DTO → domain model (BFF path — no cityId/threshold available). */
fun AlertResponseDto.toDomain(): Alert = Alert(
    id          = id,
    metal       = metal,
    targetPrice = targetPrice,
    direction   = direction,
    active      = active,
)

// ═══════════════════════════════════════════════════════════════════════════
// HomeResponse ↔ HomeEntity ↔ HomeData
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Serialises the BFF response to a singleton HomeEntity row.
 * [_degraded] is intentionally NOT persisted — it is transient only.
 */
fun HomeResponse.toEntity(cachedAt: Long = System.currentTimeMillis()): HomeEntity = HomeEntity(
    id              = 1,
    alertsJson      = json.encodeToString(alerts ?: emptyList()),
    shopSummaryJson = null, // Phase 2: marketplace shop summary
    cachedAt        = cachedAt,
)

/**
 * Parses a cached HomeEntity back to a list of Alert domain models.
 * Called by HomeRepository when rebuilding HomeData from Room cache.
 * Returns empty list on parse failure — degraded state is handled upstream.
 */
fun HomeEntity.toAlertList(): List<Alert> = runCatching {
    json.decodeFromString<List<AlertResponseDto>>(alertsJson).map { it.toDomain() }
}.getOrDefault(emptyList())

/** Full BFF response → domain model. */
fun HomeResponse.toDomain(): HomeData = HomeData(
    rate   = rates.toDomain(),
    alerts = alerts?.map { it.toDomain() } ?: emptyList(),
)
