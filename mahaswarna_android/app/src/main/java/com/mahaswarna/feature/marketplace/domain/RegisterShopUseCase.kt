package com.mahaswarna.feature.marketplace.domain

import com.mahaswarna.core.auth.JwtParser
import com.mahaswarna.core.auth.TokenStore
import com.mahaswarna.feature.billing.domain.SubscriptionTier
import com.mahaswarna.feature.marketplace.data.MarketplaceRepository
import javax.inject.Inject

/**
 * Registers a new shop for the authenticated PREMIUM user.
 *
 * Client-side guards (in addition to server enforcement):
 *   1. Tier check from JWT via [JwtParser.parse] — [TokenStore] has no getTier().
 *      Tier is decoded from the access-token claims (non-verifying, convenience only).
 *   2. GSTIN format validation (15-char regex, blank = optional).
 *
 * Server also rejects FREE-tier with 403 and duplicate shop with 409.
 */
class RegisterShopUseCase @Inject constructor(
    private val repository: MarketplaceRepository,
    private val tokenStore: TokenStore,
) {
    sealed class RegisterResult {
        data class Success(val shop: Shop) : RegisterResult()
        data object NotPremium : RegisterResult()
        data object AlreadyRegistered : RegisterResult()
        data class InvalidGstin(val message: String) : RegisterResult()
        data class Failure(val message: String) : RegisterResult()
    }

    private val gstinRe = Regex("""^[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z][1-9A-Z]Z[0-9A-Z]$""")

    suspend operator fun invoke(input: RegisterShopInput): RegisterResult {
        // Decode tier from JWT claims — JwtParser is non-verifying (server is truth)
        val tier = tokenStore.getAccessToken()
            ?.let { JwtParser.parse(it)?.tier }
            ?.let { SubscriptionTier.fromString(it) }
            ?: SubscriptionTier.FREE

        if (tier != SubscriptionTier.PREMIUM && tier != SubscriptionTier.ADMIN) {
            return RegisterResult.NotPremium
        }

        val gst = input.gstNumber.trim().uppercase()
        if (gst.isNotBlank() && !gstinRe.matches(gst)) {
            return RegisterResult.InvalidGstin("GSTIN format invalid. Expected 15-character format.")
        }

        return runCatching {
            repository.registerShop(input.copy(gstNumber = gst))
        }.fold(
            onSuccess  = { RegisterResult.Success(it) },
            onFailure  = { e ->
                when {
                    e.message?.contains("already") == true ||
                    (e is retrofit2.HttpException && e.code() == 409) ->
                        RegisterResult.AlreadyRegistered
                    else -> RegisterResult.Failure(e.message ?: "Registration failed")
                }
            },
        )
    }
}
