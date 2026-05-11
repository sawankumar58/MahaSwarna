package com.mahaswarna.local.entity

import androidx.room.Entity
import androidx.room.PrimaryKey

/** Catalog offline cache — one row per design item. */
@Entity(tableName = "designs")
data class DesignEntity(
    @PrimaryKey val id: String,
    val title: String,
    val metal: String,                 // "gold" | "silver" | "both"
    val weightGrams: Double,
    val imageUrl: String,
    val cdnKey: String,
    val cachedAt: Long,                // epoch ms
)
