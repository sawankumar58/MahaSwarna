package com.mahaswarna.local.entity

import androidx.room.Entity
import androidx.room.PrimaryKey

/**
 * Serialised snapshot of HomeResponse. One row per user session; upserted on every BFF response.
 * _degraded is transient — NOT persisted (see HomeViewModel).
 */
@Entity(tableName = "home")
data class HomeEntity(
    @PrimaryKey val id: Int = 1,      // singleton row
    val alertsJson: String,            // JSON array of AlertDto
    val shopSummaryJson: String?,      // JSON or null when no shop registered
    val cachedAt: Long,                // epoch ms
)
