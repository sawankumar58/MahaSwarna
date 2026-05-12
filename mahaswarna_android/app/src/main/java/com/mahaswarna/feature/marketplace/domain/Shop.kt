package com.mahaswarna.feature.marketplace.domain

/**
 * Domain model for a jeweller's registered shop profile.
 *
 * PREMIUM gate: Only users with [SubscriptionTier.PREMIUM] may register a shop.
 * Enforced server-side; client must also check tier before showing RegisterShop.
 *
 * [bannerUrl] is null until the jeweller uploads and the backend confirms S3 moderation passes.
 */
data class Shop(
    val id: String,
    val userId: String,
    val name: String,
    val address: String,
    val gstNumber: String,
    val phone: String,
    val bannerUrl: String? = null,
    val bannerObjectKey: String? = null,
    val createdAt: String,   // ISO-8601
    val updatedAt: String,
)

/** Input for shop registration — validated client-side before submit. */
data class RegisterShopInput(
    val name: String,
    val address: String,
    val gstNumber: String,
    val phone: String,
)
