package com.mahaswarna.feature.billing.ui

import android.app.Activity
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.android.billingclient.api.Purchase
import com.mahaswarna.feature.billing.data.BillingRepository
import com.mahaswarna.feature.billing.domain.SubscriptionTier
import com.mahaswarna.feature.billing.integrity.PlayIntegrityVerifier
import com.google.firebase.analytics.FirebaseAnalytics
import com.google.firebase.analytics.ktx.analytics
import com.google.firebase.ktx.Firebase
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

// ── UI State ──────────────────────────────────────────────────────────────────

sealed class PaywallUiState {
    data object Loading : PaywallUiState()
    data object ReadyToPurchase : PaywallUiState()
    data object Verifying : PaywallUiState()
    data class Success(val tier: SubscriptionTier) : PaywallUiState()
    data class Error(val message: String, val isRetryable: Boolean = true) : PaywallUiState()
    /** 404 from /billing/restore — no subscription found. */
    data object NoSubscriptionFound : PaywallUiState()
}

@HiltViewModel
class PaywallViewModel @Inject constructor(
    private val billingRepository: BillingRepository,
    private val integrityVerifier: PlayIntegrityVerifier,
) : ViewModel() {

    private val _uiState = MutableStateFlow<PaywallUiState>(PaywallUiState.Loading)
    val uiState: StateFlow<PaywallUiState> = _uiState.asStateFlow()

    val productDetails = billingRepository.productDetails

    init {
        connectAndLoad()
    }

    private fun connectAndLoad() {
        viewModelScope.launch {
            val connected = billingRepository.connect()
            if (!connected) {
                _uiState.value = PaywallUiState.Error("Play Store is unavailable. Please try again.")
                return@launch
            }
            billingRepository.loadProductDetails()
            _uiState.value = PaywallUiState.ReadyToPurchase
        }
    }

    /**
     * Initiates the purchase flow.
     * 1. Obtains Play Integrity token (attestation).
     * 2. Fires `subscription_flow_started` analytics event.
     * 3. Launches Play Billing flow.
     *
     * Purchase result is delivered via [onPurchaseResult].
     */
    fun startPurchase(activity: Activity) {
        val details = productDetails.value ?: run {
            _uiState.value = PaywallUiState.Error("Product details unavailable. Please try again.")
            return
        }

        viewModelScope.launch {
            _uiState.value = PaywallUiState.Loading
            runCatching {
                // Step 1: Play Integrity attestation (required before any purchase endpoint).
                val nonce = integrityVerifier.generateNonce()
                integrityVerifier.requestToken(nonce)
                // Token is sent to the backend as part of the purchase verification in onPurchaseResult.
                // For login-initiated purchases the token was already verified at login.
            }.onFailure { e ->
                _uiState.value = PaywallUiState.Error(
                    "Device attestation failed: ${e.message}",
                    isRetryable = true,
                )
                return@launch
            }

            // Step 2: Fire analytics BEFORE launching billing flow.
            Firebase.analytics.logEvent("subscription_flow_started", android.os.Bundle.EMPTY)

            // Step 3: Launch Play Billing sheet (result arrives in onPurchaseResult).
            _uiState.value = PaywallUiState.ReadyToPurchase
            billingRepository.launchBillingFlow(activity, details)
        }
    }

    /**
     * Called by the Activity/Screen when a purchase result arrives from Play Billing
     * via [BillingClient.PurchasesUpdatedListener].
     */
    fun onPurchaseResult(purchases: List<Purchase>) {
        if (purchases.isEmpty()) {
            _uiState.value = PaywallUiState.ReadyToPurchase
            return
        }
        viewModelScope.launch {
            _uiState.value = PaywallUiState.Verifying
            runCatching {
                val tier = billingRepository.verifyPurchase(purchases.first())
                // Fire analytics after successful server verification.
                Firebase.analytics.logEvent("subscription_verified", android.os.Bundle.EMPTY)
                tier
            }.onSuccess { tier ->
                _uiState.value = PaywallUiState.Success(tier)
            }.onFailure { e ->
                _uiState.value = PaywallUiState.Error(
                    e.message ?: "Purchase verification failed. Please contact support.",
                    isRetryable = false,
                )
            }
        }
    }

    /**
     * Restores an existing subscription (for reinstall / device switch).
     * Hidden when killSwitchPayments is active — caller must gate this.
     */
    fun restoreSubscription() {
        viewModelScope.launch {
            _uiState.value = PaywallUiState.Verifying
            runCatching {
                billingRepository.restoreSubscription()
            }.onSuccess { tier ->
                _uiState.value = PaywallUiState.Success(tier)
            }.onFailure { e ->
                val isNotFound = e is retrofit2.HttpException && e.code() == 404
                _uiState.value = if (isNotFound) {
                    PaywallUiState.NoSubscriptionFound
                } else {
                    PaywallUiState.Error(
                        e.message ?: "Restore failed. Please try again.",
                        isRetryable = true,
                    )
                }
            }
        }
    }

    fun retry() { connectAndLoad() }
}
