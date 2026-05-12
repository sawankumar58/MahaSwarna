package com.mahaswarna.feature.billing.domain

import com.mahaswarna.feature.billing.data.BillingRepository
import javax.inject.Inject

/**
 * Restores an existing subscription for reinstall / device-switch scenarios.
 *
 * Calls POST /billing/restore with an idempotency key. The backend checks the
 * Google Play Developer API for an active subscription tied to the authenticated user.
 *
 * On success: JWT is refreshed with the restored tier claim as a side effect.
 * Throws [retrofit2.HttpException] with 404 when no active subscription is found —
 * callers must handle this as [PaywallUiState.NoSubscriptionFound], not as a generic error.
 *
 * Hidden in [PaywallScreen] when killSwitchPayments is active.
 */
class RestoreSubscriptionUseCase @Inject constructor(
    private val billingRepository: BillingRepository,
) {
    suspend operator fun invoke(): SubscriptionTier = billingRepository.restoreSubscription()
}
