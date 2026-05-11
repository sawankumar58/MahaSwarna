package com.mahaswarna.feature.alerts.domain

/**
 * Domain model for a price threshold alert.
 *
 * [metal]       — "gold" | "silver" (not a closed enum — pass through verbatim).
 * [direction]   — "above" | "below".
 * [targetPrice] — price per gram in INR at which the alert fires.
 * [active]      — false once the alert has been triggered and delivered.
 */
data class Alert(
    val id: String,
    val metal: String,
    val targetPrice: Double,
    val direction: String,
    val active: Boolean,
)
