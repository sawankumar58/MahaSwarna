package com.mahaswarna.local.entity

import androidx.room.Entity
import androidx.room.PrimaryKey

@Entity(tableName = "alerts")
data class AlertEntity(
    @PrimaryKey val id: String,        // server-assigned UUID
    val cityId: String,
    val metal: String,                 // "gold" | "silver"
    val direction: String,             // "above" | "below"
    val threshold: Float,
    val active: Boolean,
    val createdAt: String,             // ISO-8601
)
