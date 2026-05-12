package com.mahaswarna.feature.marketplace.data

import com.mahaswarna.feature.marketplace.domain.PresignedUploadUrl
import com.mahaswarna.feature.marketplace.domain.RegisterShopInput
import com.mahaswarna.feature.marketplace.domain.Shop
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton

/**
 * MarketplaceRepository — delegates to [MarketplaceApi].
 *
 * No local Room cache for marketplace data; shops are user-specific and
 * always fetched from the network. Invoice PDFs are transient and never cached.
 *
 * All mutating calls use a UUID idempotency key to guard against double-submission
 * on network retry (server performs INSERT ON CONFLICT DO NOTHING for shops).
 */
@Singleton
class MarketplaceRepository @Inject constructor(
    private val api: MarketplaceApi,
) {

    suspend fun registerShop(input: RegisterShopInput): Shop {
        val idempotencyKey = UUID.randomUUID().toString()
        val dto = api.registerShop(
            idempotencyKey = idempotencyKey,
            request = RegisterShopRequest(
                name      = input.name,
                address   = input.address,
                gstNumber = input.gstNumber,
                phone     = input.phone,
            ),
        )
        return dto.toDomain()
    }

    suspend fun listShops(): List<Shop> =
        api.listShops().shops.map { it.toDomain() }

    suspend fun getShop(shopId: String): Shop =
        api.getShop(shopId).toDomain()

    suspend fun getBannerUploadUrl(shopId: String): PresignedUploadUrl {
        val dto = api.getBannerUploadUrl(shopId)
        return PresignedUploadUrl(
            uploadUrl   = dto.uploadUrl,
            objectKey   = dto.objectKey,
            contentType = dto.contentType,
        )
    }

    suspend fun confirmBannerUpload(shopId: String, objectKey: String) {
        api.confirmBannerUpload(shopId, BannerConfirmRequest(objectKey))
    }

    /**
     * Generates an invoice PDF.
     * ADR-001: PDF bytes NOT stored server-side.
     * Returns raw bytes for the client to display or share.
     */
    suspend fun generateInvoice(request: InvoiceRequest): ByteArray {
        val shopId = request.shopId
        val idempotencyKey = UUID.randomUUID().toString()
        val responseBody = api.generateInvoice(
            shopId         = shopId,
            idempotencyKey = idempotencyKey,
            request        = request,
        )
        return responseBody.bytes()
    }

    // ── Mapper ─────────────────────────────────────────────────────────────────

    private fun ShopDto.toDomain() = Shop(
        id              = id,
        userId          = userId,
        name            = name,
        address         = address,
        gstNumber       = gstNumber,
        phone           = phone,
        bannerUrl       = bannerUrl,
        bannerObjectKey = bannerObjectKey,
        createdAt       = createdAt,
        updatedAt       = updatedAt,
    )
}
