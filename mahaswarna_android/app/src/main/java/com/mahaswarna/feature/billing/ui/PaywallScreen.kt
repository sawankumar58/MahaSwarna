package com.mahaswarna.feature.billing.ui

import android.app.Activity
import android.view.WindowManager
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel

/**
 * Paywall screen gating premium features.
 *
 * REQUIRED: FLAG_SECURE is applied via DisposableEffect to prevent screenshots
 * of pricing UI. clearFlags in onDispose is mandatory — failing to clear leaves
 * FLAG_SECURE active on all subsequent screens until Activity recreated.
 *
 * [killSwitchPayments]: when true, this screen must not be reachable.
 * Gate navigation in the caller before routing here.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PaywallScreen(
    onNavigateBack: () -> Unit,
    onSubscribed: () -> Unit,
    viewModel: PaywallViewModel = hiltViewModel(),
) {
    val context = LocalContext.current
    val uiState by viewModel.uiState.collectAsState()
    val productDetails by viewModel.productDetails.collectAsState()

    // ── FLAG_SECURE (REQUIRED) ─────────────────────────────────────────────
    DisposableEffect(Unit) {
        val window = (context as Activity).window
        window.addFlags(WindowManager.LayoutParams.FLAG_SECURE)
        onDispose {
            window.clearFlags(WindowManager.LayoutParams.FLAG_SECURE)
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Go Premium") },
                navigationIcon = {
                    IconButton(onClick = onNavigateBack) {
                        Icon(Icons.Default.ArrowBack, contentDescription = "Back")
                    }
                }
            )
        }
    ) { innerPadding ->
        Box(
            modifier = Modifier
                .fillMaxSize()
                .padding(innerPadding)
                .padding(24.dp),
        ) {
            when (val state = uiState) {
                is PaywallUiState.Loading,
                is PaywallUiState.Verifying -> {
                    CircularProgressIndicator(modifier = Modifier.align(Alignment.Center))
                }

                is PaywallUiState.Success -> {
                    // Navigate out on next recomposition.
                    onSubscribed()
                }

                is PaywallUiState.NoSubscriptionFound -> {
                    Column(
                        modifier = Modifier.align(Alignment.Center),
                        horizontalAlignment = Alignment.CenterHorizontally,
                        verticalArrangement = Arrangement.spacedBy(12.dp),
                    ) {
                        Text(
                            "No active subscription found for this account.",
                            style = MaterialTheme.typography.bodyLarge,
                            textAlign = TextAlign.Center,
                        )
                        TextButton(onClick = { viewModel.retry() }) { Text("Try again") }
                    }
                }

                is PaywallUiState.Error -> {
                    Column(
                        modifier = Modifier.align(Alignment.Center),
                        horizontalAlignment = Alignment.CenterHorizontally,
                        verticalArrangement = Arrangement.spacedBy(12.dp),
                    ) {
                        Text(
                            state.message,
                            style = MaterialTheme.typography.bodyLarge,
                            color = MaterialTheme.colorScheme.error,
                            textAlign = TextAlign.Center,
                        )
                        if (state.isRetryable) {
                            Button(onClick = { viewModel.retry() }) { Text("Retry") }
                        }
                    }
                }

                is PaywallUiState.ReadyToPurchase -> {
                    Column(
                        modifier = Modifier.fillMaxSize(),
                        verticalArrangement = Arrangement.SpaceBetween,
                    ) {
                        // ── Value proposition ──────────────────────────────
                        Column(verticalArrangement = Arrangement.spacedBy(16.dp)) {
                            Text(
                                text = "MahaSwarna Premium",
                                style = MaterialTheme.typography.headlineMedium,
                            )
                            Text(
                                text = "Unlock AI-powered rate analysis, unlimited price alerts, and priority support.",
                                style = MaterialTheme.typography.bodyLarge,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )

                            productDetails?.let { details ->
                                val offer = details.subscriptionOfferDetails?.firstOrNull()
                                val price = offer?.pricingPhases?.pricingPhaseList
                                    ?.firstOrNull()?.formattedPrice
                                if (price != null) {
                                    Text(
                                        text = "$price / month",
                                        style = MaterialTheme.typography.titleLarge,
                                        color = MaterialTheme.colorScheme.primary,
                                    )
                                }
                            }
                        }

                        // ── CTA ────────────────────────────────────────────
                        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                            Button(
                                onClick = { viewModel.startPurchase(context as Activity) },
                                modifier = Modifier.fillMaxWidth(),
                                enabled = productDetails != null,
                            ) {
                                Text("Subscribe Now")
                            }

                            TextButton(
                                onClick = { viewModel.restoreSubscription() },
                                modifier = Modifier.fillMaxWidth(),
                            ) {
                                Text("Restore purchases")
                            }

                            Spacer(Modifier.height(8.dp))

                            Text(
                                text = "Subscription auto-renews monthly. Cancel anytime via Google Play.",
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.outline,
                                textAlign = TextAlign.Center,
                                modifier = Modifier.fillMaxWidth(),
                            )
                        }
                    }
                }
            }
        }
    }
}
