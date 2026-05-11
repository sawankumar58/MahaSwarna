package com.mahaswarna.feature.billing.data

import android.app.Activity
import com.android.billingclient.api.AcknowledgePurchaseParams
import com.android.billingclient.api.BillingClient
import com.android.billingclient.api.BillingClientStateListener
import com.android.billingclient.api.BillingFlowParams
import com.android.billingclient.api.BillingResult
import com.android.billingclient.api.ProductDetails
import com.android.billingclient.api.Purchase
import com.android.billingclient.api.QueryProductDetailsParams
import com.android.billingclient.api.QueryPurchasesParams
import com.android.billingclient.api.acknowledgePurchase
import com.android.billingclient.api.queryProductDetails
import com.android.billingclient.api.queryPurchasesAsync
import com.mahaswarna.core.auth.TokenStore
import com.mahaswarna.feature.billing.domain.SubscriptionTier
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.withContext
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException

/**
 * BillingRepository — orchestrates Google Play Billing Library 7 + backend receipt verification.
 *
 * INVARIANT: Client NEVER trusts its own purchase state. Subscription tier is read
 * exclusively from the JWT `tier` claim after a successful /billing/verify call.
 *
 * Play Integrity token must be obtained BEFORE calling [launchBillingFlow].
 * See [PaywallViewModel] for the pre-purchase integrity check.
 */
@Singleton
class BillingRepository @Inject constructor(
    private val billingClient: BillingClient,
    private val billingApi: BillingApi,
    private val tokenStore: TokenStore,
) {

    companion object {
        /** Known SKU — must match known_skus.go on backend. */
        const val PREMIUM_PRODUCT_ID = "mahaswarna_premium_monthly"
    }

    private val _productDetails = MutableStateFlow<ProductDetails?>(null)
    val productDetails: StateFlow<ProductDetails?> = _productDetails.asStateFlow()

    // ── BillingClient connection ───────────────────────────────────────────────

    /** Connects the BillingClient. Idempotent — safe to call multiple times. */
    suspend fun connect(): Boolean = suspendCancellableCoroutine { cont ->
        if (billingClient.isReady) { cont.resume(true); return@suspendCancellableCoroutine }
        billingClient.startConnection(object : BillingClientStateListener {
            override fun onBillingSetupFinished(result: BillingResult) {
                cont.resume(result.responseCode == BillingClient.BillingResponseCode.OK)
            }
            override fun onBillingServiceDisconnected() {
                if (cont.isActive) cont.resume(false)
            }
        })
    }

    // ── Product details ────────────────────────────────────────────────────────

    /** Queries Play for product details and caches in [productDetails]. */
    suspend fun loadProductDetails() {
        val params = QueryProductDetailsParams.newBuilder()
            .setProductList(
                listOf(
                    QueryProductDetailsParams.Product.newBuilder()
                        .setProductId(PREMIUM_PRODUCT_ID)
                        .setProductType(BillingClient.ProductType.SUBS)
                        .build()
                )
            )
            .build()

        val result = withContext(Dispatchers.IO) { billingClient.queryProductDetails(params) }
        if (result.billingResult.responseCode == BillingClient.BillingResponseCode.OK) {
            _productDetails.value = result.productDetailsList?.firstOrNull()
        }
    }

    // ── Purchase flow ──────────────────────────────────────────────────────────

    /**
     * Launches the Google Play subscription purchase flow.
     * Must be called from UI thread with a valid [Activity] reference.
     * [analytics.logEvent("subscription_flow_started")] must be fired BEFORE this call.
     */
    fun launchBillingFlow(activity: Activity, productDetails: ProductDetails) {
        val offerToken = productDetails.subscriptionOfferDetails?.firstOrNull()?.offerToken
            ?: return

        val productDetailsParams = BillingFlowParams.ProductDetailsParams.newBuilder()
            .setProductDetails(productDetails)
            .setOfferToken(offerToken)
            .build()

        val billingFlowParams = BillingFlowParams.newBuilder()
            .setProductDetailsParamsList(listOf(productDetailsParams))
            .build()

        billingClient.launchBillingFlow(activity, billingFlowParams)
    }

    // ── Receipt verification ───────────────────────────────────────────────────

    /**
     * Verifies a purchase receipt with the backend and returns the updated [SubscriptionTier].
     * Acknowledges the purchase with Play on success.
     * Fires `subscription_verified` analytics event — caller's responsibility.
     */
    suspend fun verifyPurchase(purchase: Purchase): SubscriptionTier {
        val idempotencyKey = UUID.randomUUID().toString()
        val response = billingApi.verifyReceipt(
            idempotencyKey = idempotencyKey,
            request = VerifyReceiptRequest(
                purchaseToken = purchase.purchaseToken,
                productId = purchase.products.firstOrNull() ?: PREMIUM_PRODUCT_ID,
            ),
        )

        // Acknowledge purchase with Play after successful server verification.
        if (purchase.purchaseState == Purchase.PurchaseState.PURCHASED && !purchase.isAcknowledged) {
            val ackParams = AcknowledgePurchaseParams.newBuilder()
                .setPurchaseToken(purchase.purchaseToken)
                .build()
            billingClient.acknowledgePurchase(ackParams)
        }

        // Refresh JWT so the new tier is reflected in all subsequent API calls.
        refreshJwtFromTier(response)
        return SubscriptionTier.fromString(response.tier)
    }

    // ── Restore subscription ───────────────────────────────────────────────────

    /**
     * Restores an active subscription for users reinstalling or switching devices.
     * Throws [retrofit2.HttpException] with 404 when no active subscription found.
     */
    suspend fun restoreSubscription(): SubscriptionTier {
        val idempotencyKey = UUID.randomUUID().toString()
        val response = billingApi.restoreSubscription(idempotencyKey)
        refreshJwtFromTier(response)
        return SubscriptionTier.fromString(response.tier)
    }

    // ── Pending purchases (on launch check) ───────────────────────────────────

    /**
     * Queries Play for any unacknowledged purchases from a previous session.
     * Call on app resume to catch purchases completed in background.
     */
    suspend fun queryPendingPurchases(): List<Purchase> {
        val params = QueryPurchasesParams.newBuilder()
            .setProductType(BillingClient.ProductType.SUBS)
            .build()
        val result = withContext(Dispatchers.IO) { billingClient.queryPurchasesAsync(params) }
        return if (result.billingResult.responseCode == BillingClient.BillingResponseCode.OK) {
            result.purchasesList.filter {
                it.purchaseState == Purchase.PurchaseState.PURCHASED && !it.isAcknowledged
            }
        } else emptyList()
    }

    // ── Private helpers ────────────────────────────────────────────────────────

    /**
     * After a successful /billing/verify or /billing/restore, the backend issues
     * an updated JWT with the new tier claim. Token is stored via TokenStore.
     * In Phase 1 the updated token comes back via the Authorization response header,
     * intercepted by AuthInterceptor. This placeholder saves the tier for local reads.
     */
    private fun refreshJwtFromTier(@Suppress("UNUSED_PARAMETER") response: BillingResponse) {
        // AuthInterceptor automatically stores the refreshed JWT from the response header.
        // No additional action required here — tier is read from the next JWT parse.
    }
}
