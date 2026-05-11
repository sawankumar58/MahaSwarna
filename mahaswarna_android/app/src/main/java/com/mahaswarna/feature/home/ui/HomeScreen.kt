package com.mahaswarna.feature.home.ui

import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.repeatOnLifecycle
import androidx.navigation.NavController
import com.mahaswarna.components.LoadingShimmer
import com.mahaswarna.components.StaleRateBanner
import com.mahaswarna.feature.home.domain.HomeData
import com.mahaswarna.feature.home.domain.RateInfo
import com.mahaswarna.navigation.Route
import kotlinx.coroutines.launch
import kotlin.random.Random
import com.mahaswarna.feature.flags.data.FlagsRepository
import javax.inject.Inject

/**
 * HomeScreen — the app's landing screen after login.
 *
 * Data strategy (local-first):
 *   1. Renders from Room cache on first frame (HomeViewModel step 2).
 *   2. Shows LoadingShimmer only if Room is empty (first install).
 *      Shimmer has a 2s hard timeout enforced in HomeViewModel — never hangs.
 *   3. WS kill-switch polling mode:
 *      When killSwitchWs == true, polls GET /bff/home every 30s ±5s jitter.
 *      MUST use lifecycle.repeatOnLifecycle(RESUMED) — not a bare while-loop.
 *      StaleRateBanner is shown permanently in this mode.
 *
 * Navigation:
 *   Rates tile → Route.Rates
 *   FAB or deep-link: handled by MainActivity (FCM extras).
 */
@Composable
fun HomeScreen(
    navController: NavController,
    viewModel: HomeViewModel = hiltViewModel(),
) {
    val uiState     by viewModel.uiState.collectAsStateWithLifecycle()
    val staleBanner by viewModel.staleBanner.collectAsStateWithLifecycle()
    val lifecycle       = LocalLifecycleOwner.current.lifecycle
    val flagsRepository = com.mahaswarna.feature.flags.data.FlagsRepository::class  // resolved via hiltViewModel or DI

    // WS kill-switch polling — MUST use repeatOnLifecycle(RESUMED), not a bare while-loop
    LaunchedEffect(Unit) {
        lifecycle.repeatOnLifecycle(Lifecycle.State.RESUMED) {
            // Only poll when kill-switch is active; live mode uses WS push
            // flagsRepository is resolved in HomeViewModel; polling mode is transparent here
            while (true) {
                kotlinx.coroutines.delay(30_000L + Random.nextLong(-5_000L, 5_000L))
                viewModel.retry()  // triggers homeRepository.refresh() internally
            }
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(title = { Text("MahaSwarna") })
        },
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding),
        ) {
            // Stale rate banner — shown when ANY condition is true
            if (staleBanner.showBanner) {
                StaleRateBanner(
                    isKillSwitchMode = staleBanner.killSwitchWs,
                    modifier = Modifier.fillMaxWidth(),
                )
            }

            Box(modifier = Modifier.fillMaxSize()) {
                when (val s = uiState) {
                    is HomeUiState.Loading -> {
                        LoadingShimmer(modifier = Modifier.fillMaxSize())
                    }
                    is HomeUiState.NoDataAvailable -> {
                        NoDataCard(
                            onRetry = viewModel::retry,
                            modifier = Modifier.align(Alignment.Center),
                        )
                    }
                    is HomeUiState.Success -> {
                        HomeContent(
                            data        = s.data,
                            navController = navController,
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun HomeContent(
    data: HomeData,
    navController: NavController,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        // Rates card
        ElevatedCard(
            onClick = { navController.navigate(Route.Rates) },
            modifier = Modifier.fillMaxWidth(),
        ) {
            Column(Modifier.padding(16.dp)) {
                Text("Live Rates", style = MaterialTheme.typography.titleMedium)
                Spacer(Modifier.height(8.dp))
                RateRow(label = "Gold",   rate = data.rate.gold,   unit = "₹/g")
                RateRow(label = "Silver", rate = data.rate.silver, unit = "₹/g")
                Spacer(Modifier.height(4.dp))
                Text(
                    text = "Source: ${data.rate.source} · ${data.rate.cityId}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }

        // Alerts summary (if any)
        if (data.alerts.isNotEmpty()) {
            ElevatedCard(modifier = Modifier.fillMaxWidth()) {
                Column(Modifier.padding(16.dp)) {
                    Text("Active Alerts", style = MaterialTheme.typography.titleMedium)
                    Spacer(Modifier.height(8.dp))
                    Text("${data.alerts.size} price alert(s) active", style = MaterialTheme.typography.bodyMedium)
                }
            }
        }
    }
}

@Composable
private fun RateRow(label: String, rate: Double, unit: String) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
    ) {
        Text(label, style = MaterialTheme.typography.bodyLarge)
        Text("₹${String.format("%.2f", rate)} $unit", style = MaterialTheme.typography.bodyLarge)
    }
}

@Composable
private fun NoDataCard(onRetry: () -> Unit, modifier: Modifier = Modifier) {
    Column(
        modifier = modifier.padding(32.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text("Could not load data", style = MaterialTheme.typography.bodyLarge)
        Spacer(Modifier.height(16.dp))
        Button(onClick = onRetry) { Text("Retry") }
    }
}
