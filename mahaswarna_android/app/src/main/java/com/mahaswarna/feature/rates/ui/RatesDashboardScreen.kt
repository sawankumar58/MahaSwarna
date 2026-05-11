package com.mahaswarna.feature.rates.ui

import androidx.compose.foundation.layout.*
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Calculate
import androidx.compose.material.icons.filled.History
import androidx.compose.material.icons.filled.LocationCity
import androidx.compose.material.icons.filled.Receipt
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavController
import com.google.firebase.analytics.FirebaseAnalytics
import com.google.firebase.analytics.ktx.analytics
import com.google.firebase.analytics.ktx.logEvent
import com.google.firebase.ktx.Firebase
import com.mahaswarna.components.StaleRateBanner
import com.mahaswarna.core.network.ApiConstants
import com.mahaswarna.navigation.Route

/**
 * RatesDashboardScreen — live gold/silver tiles with WS push.
 *
 * Analytics: fires rate_viewed { cityId, source } on first composition when rates available.
 * source is read from rate.source — NEVER hardcoded as "gemini".
 *
 * FAB navigation invariants:
 *   isStale is read from ratesViewModel.isStale.value at the moment of tap.
 *   Source: the live StateFlow combining rate.isStale + wsDisconnected.
 *   DO NOT read from Room's RateEntity — it lags behind live WS state.
 *
 * Back-stack:
 *   Calculator → back → RatesDashboard (NOT Home). Use navController.navigate(), not popBackStack.
 *   BillPrint  → back → RatesDashboard or Calculator.
 */
@Composable
fun RatesDashboardScreen(
    navController: NavController,
    viewModel: RatesDashboardViewModel = hiltViewModel(),
) {
    val uiState  by viewModel.uiState.collectAsStateWithLifecycle()
    val isStale  by viewModel.isStale.collectAsStateWithLifecycle()
    val selectedCity by remember { mutableStateOf(ApiConstants.DEFAULT_CITY.id) }
    var showCityPicker by remember { mutableStateOf(false) }

    // Fire rate_viewed analytics on first Success
    LaunchedEffect(uiState) {
        if (uiState is RatesDashboardUiState.Success) {
            val rate = (uiState as RatesDashboardUiState.Success).rate
            Firebase.analytics.logEvent("rate_viewed") {
                param("cityId", rate.cityId)
                param("source", rate.source)  // always derive from rate.source, never hardcode
            }
        }
    }

    // Fetch via REST if no WS data yet
    LaunchedEffect(selectedCity) {
        if (uiState is RatesDashboardUiState.Loading) {
            viewModel.fetchRate(selectedCity)
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Rates · ${ApiConstants.CITY_LIST.find { it.id == selectedCity }?.displayName ?: selectedCity}") },
                actions = {
                    IconButton(onClick = { showCityPicker = true }) {
                        Icon(Icons.Default.LocationCity, contentDescription = "Change city")
                    }
                    IconButton(onClick = { navController.navigate(Route.RateHistory) }) {
                        Icon(Icons.Default.History, contentDescription = "Rate history")
                    }
                },
            )
        },
        floatingActionButton = {
            if (uiState is RatesDashboardUiState.Success) {
                val rate = (uiState as RatesDashboardUiState.Success).rate
                FloatingActionButton(
                    onClick = {
                        // isStale read from live StateFlow at tap time — NOT from Room
                        navController.navigate(
                            Route.Calculator(
                                goldRate   = rate.gold,
                                silverRate = rate.silver,
                                isStale    = isStale,
                            )
                        )
                    },
                ) {
                    Icon(Icons.Default.Calculate, contentDescription = "Open calculator")
                }
            }
        },
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding),
        ) {
            if (isStale) {
                StaleRateBanner(modifier = Modifier.fillMaxWidth())
            }

            Box(modifier = Modifier.fillMaxSize()) {
                when (val s = uiState) {
                    is RatesDashboardUiState.Loading ->
                        CircularProgressIndicator(Modifier.align(Alignment.Center))

                    is RatesDashboardUiState.Error ->
                        Column(
                            Modifier.align(Alignment.Center).padding(32.dp),
                            horizontalAlignment = Alignment.CenterHorizontally,
                        ) {
                            Text(s.message, style = MaterialTheme.typography.bodyLarge)
                            Spacer(Modifier.height(16.dp))
                            Button(onClick = { viewModel.fetchRate(selectedCity) }) { Text("Retry") }
                        }

                    is RatesDashboardUiState.Success ->
                        RatesContent(
                            rate        = s.rate,
                            navController = navController,
                            isStale     = isStale,
                        )
                }
            }
        }
    }

    if (showCityPicker) {
        CityPickerBottomSheet(
            currentCityId  = selectedCity,
            onCitySelected = { /* update selectedCity; call viewModel.fetchRate(it) */ showCityPicker = false },
            onDismiss      = { showCityPicker = false },
        )
    }
}

@Composable
private fun RatesContent(
    rate: com.mahaswarna.feature.rates.domain.Rate,
    navController: NavController,
    isStale: Boolean,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        // Gold tile
        MetalRateCard(
            metal  = "Gold",
            rate   = rate.gold,
            unit   = "₹/g",
            source = rate.source,
            isStale = isStale,
        )
        // Silver tile
        MetalRateCard(
            metal  = "Silver",
            rate   = rate.silver,
            unit   = "₹/g",
            source = rate.source,
            isStale = isStale,
        )

        // Generate Bill button
        OutlinedButton(
            onClick = {
                navController.navigate(
                    Route.BillPrint(
                        goldRate   = rate.gold,
                        silverRate = rate.silver,
                        isStale    = isStale,  // live StateFlow value at tap time
                    )
                )
            },
            modifier = Modifier.fillMaxWidth(),
        ) {
            Icon(Icons.Default.Receipt, contentDescription = null, modifier = Modifier.size(18.dp))
            Spacer(Modifier.width(8.dp))
            Text("Generate Bill")
        }
    }
}

@Composable
private fun MetalRateCard(
    metal: String,
    rate: Double,
    unit: String,
    source: String,
    isStale: Boolean,
) {
    ElevatedCard(modifier = Modifier.fillMaxWidth()) {
        Column(Modifier.padding(20.dp)) {
            Row(
                Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(metal, style = MaterialTheme.typography.titleMedium)
                if (isStale) {
                    AssistChip(
                        onClick = {},
                        label = { Text("Stale") },
                        colors = AssistChipDefaults.assistChipColors(
                            containerColor = MaterialTheme.colorScheme.errorContainer,
                            labelColor = MaterialTheme.colorScheme.onErrorContainer,
                        ),
                    )
                }
            }
            Spacer(Modifier.height(8.dp))
            Text(
                text = "₹${String.format("%,.2f", rate)} $unit",
                style = MaterialTheme.typography.headlineMedium,
            )
            Spacer(Modifier.height(4.dp))
            Text(
                text = "Source: $source",  // source from backend — NEVER hardcode "gemini"
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}
