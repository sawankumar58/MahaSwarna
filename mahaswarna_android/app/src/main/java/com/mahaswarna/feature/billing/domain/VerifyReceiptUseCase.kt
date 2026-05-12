package com.mahaswarna.feature.billing.domain

import com.android.billingclient.api.Purchase
import com.mahaswarna.feature.billing.data.BillingRepository
import javax.inject.Inject

/**
 * Verifies a completed Play Billing purchase with the backend.
 *
 * Flow:
 *   1. Sends [Purchase.purchaseToken] to POST /billing/verify with an idempotency key.
 *   2. Backend validates with Google Play Developer API.
 *   3. On success: acknowledges the purchase with Play and returns the new [SubscriptionTier].
 *   4. JWT is refreshed with the updated tier claim as a side effect.
 *
 * Throws [retrofit2.HttpException] on server-side verification failure.
 * Throws [com.android.billingclient.api.BillingException] on acknowledgement failure.
 *
 * INVARIANT: never call this with an already-acknowledged purchase — the idempotency
 * key on the server prevents double-billing, but it's an unnecessary network round trip.
 */
class VerifyReceiptUseCase @Inject constructor(
    private val billingRepository: BillingRepository,
) {
    suspend operator fun invoke(purchase: Purchase): SubscriptionTier =
        billingRepository.verifyPurchase(purchase)
}
