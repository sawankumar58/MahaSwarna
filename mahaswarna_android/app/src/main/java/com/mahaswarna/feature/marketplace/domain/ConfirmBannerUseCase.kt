package com.mahaswarna.feature.marketplace.domain

import com.mahaswarna.feature.marketplace.data.MarketplaceRepository
import javax.inject.Inject

/**
 * Confirms a banner upload after the client has PUT the image to the S3 presigned URL.
 *
 * The backend will:
 *   1. Verify the object exists in S3 at [objectKey].
 *   2. Run async image moderation (AI Safety / AWS Rekognition).
 *   3. Set shop.banner_url only if moderation passes.
 *
 * This call returns immediately once the server has enqueued moderation.
 * The client should poll GET /marketplace/shops/{shopId} to see the final banner_url.
 *
 * On moderation failure the server sends a push notification and leaves banner_url null.
 */
class ConfirmBannerUseCase @Inject constructor(
    private val repository: MarketplaceRepository,
) {
    suspend operator fun invoke(shopId: String, objectKey: String): Result<Unit> =
        runCatching { repository.confirmBannerUpload(shopId, objectKey) }
}
