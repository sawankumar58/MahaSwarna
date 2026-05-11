package com.mahaswarna.feature.billing.domain

/**
 * Subscription tier parsed from the JWT `tier` claim.
 *
 * INVARIANT: Client NEVER derives tier from local purchase state.
 * Tier is set exclusively from the JWT after a successful /billing/verify or /billing/restore.
 *
 * [fromString] is lenient: unknown values map to FREE so new server-added tiers
 * don't crash existing clients.
 */
enum class SubscriptionTier {
    FREE,
    PREMIUM,
    ADMIN;

    companion object {
        fun fromString(value: String): SubscriptionTier = when (value.uppercase()) {
            "PREMIUM" -> PREMIUM
            "ADMIN"   -> ADMIN
            else      -> FREE
        }
    }
}
