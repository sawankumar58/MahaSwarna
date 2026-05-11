package com.mahaswarna.local.entity

import androidx.room.Entity
import androidx.room.PrimaryKey

@Entity(tableName = "rates")
data class RateEntity(
    @PrimaryKey val cityId: String,
    val gold: Double,
    val silver: Double,
    val source: String,
    val generatedAt: String,   // ISO-8601
    val isStale: Boolean,       // from backend — NEVER computed from cachedAt
    val cachedAt: Long,         // local epoch ms
)
