package com.mahaswarna.feature.marketplace.domain

import com.mahaswarna.feature.marketplace.data.MarketplaceRepository
import javax.inject.Inject

/**
 * Requests a pre-signed S3 upload URL for a shop banner image.
 *
 * Flow:
 *   1. Client calls this use case → receives [PresignedUploadUrl].
 *   2. Client PUTs the image bytes directly to [PresignedUploadUrl.uploadUrl] (no auth header).
 *   3. Client calls [ConfirmBannerUseCase] with [PresignedUploadUrl.objectKey].
 *   4. Backend moderates the image (async); [bannerUrl] becomes non-null on success.
 *
 * The presigned URL expires in 15 minutes (enforced server-side).
 * Content-Type must match [PresignedUploadUrl.contentType] in the PUT request.
 */
class GetBannerUploadUrlUseCase @Inject constructor(
    private val repository: MarketplaceRepository,
) {
    suspend operator fun invoke(shopId: String): Result<PresignedUploadUrl> =
        runCatching { repository.getBannerUploadUrl(shopId) }
}

data class PresignedUploadUrl(
    val uploadUrl: String,    // S3 presigned PUT URL (expires in 15 min)
    val objectKey: String,    // S3 key to pass to ConfirmBannerUseCase
    val contentType: String,  // e.g. "image/jpeg" — must be set in the PUT Content-Type header
)
