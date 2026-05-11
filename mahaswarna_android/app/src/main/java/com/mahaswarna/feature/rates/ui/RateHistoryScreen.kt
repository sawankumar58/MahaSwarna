package com.mahaswarna.feature.rates.ui

import androidx.compose.foundation.layout.*
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavController
import com.mahaswarna.core.network.ApiConstants
import com.patrykandpatrick.vico.compose.cartesian.CartesianChartHost
import com.patrykandpatrick.vico.compose.cartesian.axis.rememberBottomAxis
import com.patrykandpatrick.vico.compose.cartesian.axis.rememberStartAxis
import com.patrykandpatrick.vico.compose.cartesian.layer.rememberLineCartesianLayer
import com.patrykandpatrick.vico.compose.cartesian.rememberCartesianChart
import com.patrykandpatrick.vico.core.cartesian.data.CartesianChartModelProducer
import com.patrykandpatrick.vico.core.cartesian.data.lineSeries

/**
 * RateHistoryScreen — GAP-M4 fix.
 *
 * Displays gold and silver rate history for the selected city as a Vico line chart.
 * Uses com.patrykandpatrick.vico:compose-m3 (already in build.gradle.kts as `vico.compose.m3`).
 *
 * X-axis: timestamps (IST); Y-axis: INR per gram.
 * INR formatting: Locale("en", "IN").
 *
 * cityId is passed from RatesDashboardScreen (or read from session preference).
 * Navigation: accessible from RatesDashboard via "View history" action icon.
 *
 * No Room cache — history is always fetched from network.
 * Error state shows a Retry button.
 */
@Composable
fun RateHistoryScreen(
    cityId: String,
    navController: NavController,
    viewModel: RateHistoryViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()

    LaunchedEffect(cityId) {
        viewModel.loadHistory(cityId)
    }

    val cityDisplayName = ApiConstants.CITY_LIST.find { it.id == cityId }?.displayName ?: cityId

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Rate History · $cityDisplayName") },
                navigationIcon = {
                    IconButton(onClick = { navController.popBackStack() }) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                },
            )
        },
    ) { padding ->
        Box(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding),
        ) {
            when (val s = uiState) {
                is RateHistoryUiState.Loading ->
                    CircularProgressIndicator(Modifier.align(Alignment.Center))

                is RateHistoryUiState.Error ->
                    Column(
                        Modifier.align(Alignment.Center).padding(32.dp),
                        horizontalAlignment = Alignment.CenterHorizontally,
                    ) {
                        Text(s.message, style = MaterialTheme.typography.bodyLarge)
                        Spacer(Modifier.height(16.dp))
                        Button(onClick = { viewModel.retry(cityId) }) { Text("Retry") }
                    }

                is RateHistoryUiState.Success -> {
                    RateHistoryChart(
                        points  = s.points,
                        modifier = Modifier
                            .fillMaxSize()
                            .padding(16.dp),
                    )
                }
            }
        }
    }
}

@Composable
private fun RateHistoryChart(
    points: List<com.mahaswarna.feature.rates.domain.RateHistoryPoint>,
    modifier: Modifier = Modifier,
) {
    val modelProducer = remember { CartesianChartModelProducer() }

    LaunchedEffect(points) {
        modelProducer.runTransaction {
            lineSeries {
                // Gold series (index 0)
                series(y = points.map { it.gold.toFloat() })
            }
            lineSeries {
                // Silver series (index 1)
                series(y = points.map { it.silver.toFloat() })
            }
        }
    }

    Column(modifier = modifier) {
        // Legend
        Row(horizontalArrangement = Arrangement.spacedBy(16.dp)) {
            LegendDot(color = MaterialTheme.colorScheme.primary,   label = "Gold (₹/g)")
            LegendDot(color = MaterialTheme.colorScheme.secondary, label = "Silver (₹/g)")
        }
        Spacer(Modifier.height(12.dp))

        CartesianChartHost(
            chart = rememberCartesianChart(
                rememberLineCartesianLayer(),
                rememberLineCartesianLayer(),
                startAxis = rememberStartAxis(),
                bottomAxis = rememberBottomAxis(),
            ),
            modelProducer = modelProducer,
            modifier = Modifier
                .fillMaxWidth()
                .height(300.dp),
        )

        // X-axis labels (timestamps) — displayed below chart as scrollable text if needed
        if (points.isNotEmpty()) {
            Spacer(Modifier.height(8.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
            ) {
                Text(
                    text = formatIst(points.first().generatedAt),
                    style = MaterialTheme.typography.labelSmall,
                )
                Text(
                    text = formatIst(points.last().generatedAt),
                    style = MaterialTheme.typography.labelSmall,
                )
            }
        }
    }
}

@Composable
private fun LegendDot(color: Color, label: String) {
    Row(verticalAlignment = Alignment.CenterVertically) {
        Surface(
            color  = color,
            shape  = MaterialTheme.shapes.small,
            modifier = Modifier.size(12.dp),
        ) {}
        Spacer(Modifier.width(4.dp))
        Text(label, style = MaterialTheme.typography.labelSmall)
    }
}

/** Format ISO-8601 IST timestamp to "dd MMM HH:mm" for X-axis labels. */
private fun formatIst(iso: String): String = runCatching {
    val odt = java.time.OffsetDateTime.parse(iso)
    val ist = odt.atZoneSameInstant(java.time.ZoneId.of("Asia/Kolkata"))
    ist.format(java.time.format.DateTimeFormatter.ofPattern("dd MMM HH:mm"))
}.getOrElse { iso.take(16) }
