package com.mahaswarna.feature.marketplace.data

import kotlinx.serialization.Serializable
import okhttp3.ResponseBody
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.Header
import retrofit2.http.POST
import retrofit2.http.PUT
import retrofit2.http.Path
import retrofit2.http.Streaming

// ── Request DTOs ──────────────────────────────────────────────────────────────

@Serializable
data class RegisterShopRequest(
    val name: String,
    val address: String,
    val gstNumber: String,
    val phone: String,
)

@Serializable
data class BannerConfirmRequest(
    val objectKey: String,
)

@Serializable
data class InvoiceLineItemDto(
    val description: String,
    val weightGrams: Double,
    val karat: Int = 22,
    val makingCharge: Double = 0.0,
)

@Serializable
data class InvoiceRequest(
    val shopId: String,
    val customerName: String,
    val customerPhone: String? = null,
    val items: List<InvoiceLineItemDto>,
    /** "cash" | "upi" | "card" */
    val paymentMode: String = "cash",
    val notes: String? = null,
    /**
     * Client-supplied rate override (from live rate at time of generation).
     * If null, the backend fetches the current rate from Pricing service.
     */
    val goldRateOverride: Double? = null,
    val silverRateOverride: Double? = null,
)

// ── Response DTOs ─────────────────────────────────────────────────────────────

@Serializable
data class ShopDto(
    val id: String,
    val userId: String,
    val name: String,
    val address: String,
    val gstNumber: String,
    val phone: String,
    val bannerUrl: String? = null,
    val bannerObjectKey: String? = null,
    val createdAt: String,
    val updatedAt: String,
)

@Serializable
data class ShopListDto(
    val shops: List<ShopDto>,
)

@Serializable
data class BannerUploadUrlDto(
    val uploadUrl: String,
    val objectKey: String,
    val contentType: String = "image/jpeg",
)

// ── Retrofit interface ─────────────────────────────────────────────────────────

interface MarketplaceApi {

    /**
     * POST /v1/marketplace/shops
     * Registers a new shop. PREMIUM tier only; 403 otherwise.
     * 409 if user already has a shop.
     * Idempotency-Key required to prevent double-submission on retry.
     */
    @POST("marketplace/shops")
    suspend fun registerShop(
        @Header("Idempotency-Key") idempotencyKey: String,
        @Body request: RegisterShopRequest,
    ): ShopDto

    /**
     * GET /v1/marketplace/shops
     * Lists all shops for the authenticated user (usually 0 or 1).
     */
    @GET("marketplace/shops")
    suspend fun listShops(): ShopListDto

    /**
     * GET /v1/marketplace/shops/{shopId}
     * Returns the shop profile. Poll after banner upload to check moderation status.
     */
    @GET("marketplace/shops/{shopId}")
    suspend fun getShop(@Path("shopId") shopId: String): ShopDto

    /**
     * POST /v1/marketplace/shops/{shopId}/banner-upload-url
     * Returns a pre-signed S3 PUT URL valid for 15 minutes.
     */
    @POST("marketplace/shops/{shopId}/banner-upload-url")
    suspend fun getBannerUploadUrl(
        @Path("shopId") shopId: String,
    ): BannerUploadUrlDto

    /**
     * POST /v1/marketplace/shops/{shopId}/banner-confirm
     * Enqueues server-side image moderation for the uploaded banner.
     * Returns 202 Accepted; banner_url is populated asynchronously.
     */
    @POST("marketplace/shops/{shopId}/banner-confirm")
    suspend fun confirmBannerUpload(
        @Path("shopId") shopId: String,
        @Body request: BannerConfirmRequest,
    )

    /**
     * POST /v1/marketplace/shops/{shopId}/invoices
     * Generates an invoice PDF and streams it back as application/pdf.
     * ADR-001: PDF bytes NOT stored server-side; client receives raw bytes.
     * HTTP 429 when daily invoice quota (60) exceeded.
     */
    @Streaming
    @POST("marketplace/shops/{shopId}/invoices")
    suspend fun generateInvoice(
        @Path("shopId") shopId: String,
        @Header("Idempotency-Key") idempotencyKey: String,
        @Body request: InvoiceRequest,
    ): ResponseBody
}
