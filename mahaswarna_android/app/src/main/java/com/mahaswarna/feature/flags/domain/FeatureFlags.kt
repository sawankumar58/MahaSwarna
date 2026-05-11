package com.mahaswarna.feature.flags.domain

/**
 * Domain model for feature flags fetched from GET /config/feature-flags.
 *
 * Kill-switches are emergency circuit breakers:
 *   - killSwitchAi          → AI Banner Scanner disabled (Catalog, ShopBanner)
 *   - killSwitchWs          → WebSocket disabled; HomeViewModel polls REST every 30s ±5s jitter
 *   - killSwitchPayments    → IAP disabled; PaywallScreen hides restore action
 *   - killSwitchCatalog     → Catalog tab hidden
 *   - killSwitchImageSearch → Image-search route ABSENT from NavHost (default TRUE — backend not
 *                              implemented; Route.ImageSearch must NOT be registered while true)
 *
 * params: opaque numeric config bag — parsed from FlagsDto but NEVER used for client-side
 * rate filtering. rate_sanity_threshold_pct is enforced server-side only.
 *
 * DEFAULT_FLAGS: used on first install and on any network failure in FlagsRepository.
 * killSwitchImageSearch MUST be true in DEFAULT_FLAGS — omitting it allows image search
 * on first install before the backend endpoint is implemented.
 */
data class FeatureFlags(
    val aiEnabled: Boolean,
    val shopEnabled: Boolean,
    val wsEnabled: Boolean,
    val paymentsEnabled: Boolean,
    val catalogEnabled: Boolean,
    val killSwitchAi: Boolean,
    val killSwitchWs: Boolean,
    val killSwitchPayments: Boolean,
    val killSwitchCatalog: Boolean,
    val killSwitchImageSearch: Boolean,  // default true — image search backend not implemented
    val params: Map<String, Double>,
) {
    companion object {
        /**
         * Safe defaults used on first install or network failure.
         * INVARIANT: killSwitchImageSearch = true (backend endpoint does not exist yet).
         * All other kill-switches default to false (features on).
         */
        val DEFAULT = FeatureFlags(
            aiEnabled          = true,
            shopEnabled        = true,
            wsEnabled          = true,
            paymentsEnabled    = true,
            catalogEnabled     = true,
            killSwitchAi          = false,
            killSwitchWs          = false,
            killSwitchPayments    = false,
            killSwitchCatalog     = false,
            killSwitchImageSearch = true,   // ← MUST remain true until backend ships
            params = mapOf(
                "rate_sanity_threshold_pct" to 2.0,
                "rate_limit_bff_free_rpm"   to 40.0,
            ),
        )
    }
}
