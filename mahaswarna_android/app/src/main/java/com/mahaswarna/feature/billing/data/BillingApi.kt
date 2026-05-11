package com.mahaswarna.feature.billing.data

import kotlinx.serialization.Serializable
import retrofit2.http.Body
import retrofit2.http.Header
import retrofit2.http.POST

// ── Request DTOs ──────────────────────────────────────────────────────────────

@Serializable
data class VerifyReceiptRequest(
    val purchaseToken: String,
    val productId: String,
    val packageName: String = "com.mahaswarna",
)

// ── Response DTOs ─────────────────────────────────────────────────────────────

@Serializable
data class BillingResponse(
    val tier: String,         // "FREE" | "PREMIUM" | "ADMIN"
    val expiresAt: String,    // ISO-8601
)

// ── Retrofit interface ────────────────────────────────────────────────────────

interface BillingApi {

    /**
     * POST /billing/verify
     * Verifies a Google Play purchase token server-side via the Play Developer API.
     * Idempotent — re-sending the same token is safe (INSERT ON CONFLICT DO NOTHING).
     * Requires [Idempotency-Key] header to guard against double-billing on network retry.
     */
    @POST("billing/verify")
    suspend fun verifyReceipt(
        @Header("Idempotency-Key") idempotencyKey: String,
        @Body request: VerifyReceiptRequest,
    ): BillingResponse

    /**
     * POST /billing/restore
     * Queries the Play Developer API for active subscriptions linked to the Google account.
     * Returns 404 when no active subscription found.
     * Returns 503 on upstream Play API error — client shows generic retry.
     */
    @POST("billing/restore")
    suspend fun restoreSubscription(
        @Header("Idempotency-Key") idempotencyKey: String,
    ): BillingResponse
}
