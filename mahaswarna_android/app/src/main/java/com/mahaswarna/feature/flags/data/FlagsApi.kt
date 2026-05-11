package com.mahaswarna.feature.flags.data

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import retrofit2.http.GET

// ── Wire DTO ─────────────────────────────────────────────────────────────────

@Serializable
data class FlagsDto(
    val flags: FlagToggles,
    @SerialName("killSwitch") val killSwitch: KillSwitches,
    val params: Map<String, Double> = emptyMap(),
)

@Serializable
data class FlagToggles(
    @SerialName("ai_enabled")       val aiEnabled:       Boolean = true,
    @SerialName("shop_enabled")     val shopEnabled:     Boolean = true,
    @SerialName("ws_enabled")       val wsEnabled:       Boolean = true,
    @SerialName("payments_enabled") val paymentsEnabled: Boolean = true,
    @SerialName("catalog_enabled")  val catalogEnabled:  Boolean = true,
)

@Serializable
data class KillSwitches(
    val ai:           Boolean = false,
    val ws:           Boolean = false,
    val payments:     Boolean = false,
    val catalog:      Boolean = false,
    @SerialName("image_search") val imageSearch: Boolean = true,  // default true — blocked
)

// ── Retrofit interface ────────────────────────────────────────────────────────

/**
 * Client always calls /config/feature-flags (public gateway path).
 * Gateway rewrites to core:4001/flags/public internally.
 * NEVER reference the internal path in client code.
 */
interface FlagsApi {
    @GET("config/feature-flags")
    suspend fun getFeatureFlags(): FlagsDto
}
