package com.mahaswarna.core.auth

import android.util.Base64
import org.json.JSONObject

/**
 * Decodes a JWT payload without signature verification.
 * Server is the source of truth — this is client-side convenience only
 * (tier display, expiry check for pre-warm).
 */
object JwtParser {
    data class Claims(val tier: String?, val exp: Long)

    fun parse(jwt: String): Claims? = runCatching {
        val payloadB64 = jwt.split(".").getOrNull(1) ?: return null
        val decoded = Base64.decode(payloadB64, Base64.URL_SAFE or Base64.NO_PADDING)
        val json = JSONObject(String(decoded))
        Claims(
            tier = json.optString("tier").takeIf { it.isNotBlank() },
            exp  = json.getLong("exp"),
        )
    }.getOrNull()

    fun isExpired(jwt: String, clockSkewSeconds: Long = 30): Boolean {
        val claims = parse(jwt) ?: return true
        val nowSeconds = System.currentTimeMillis() / 1000
        return claims.exp <= nowSeconds + clockSkewSeconds
    }

    fun shouldRefresh(jwt: String, refreshThresholdMinutes: Long = 12): Boolean {
        val claims = parse(jwt) ?: return true
        val nowSeconds = System.currentTimeMillis() / 1000
        return claims.exp - nowSeconds <= refreshThresholdMinutes * 60
    }
}
