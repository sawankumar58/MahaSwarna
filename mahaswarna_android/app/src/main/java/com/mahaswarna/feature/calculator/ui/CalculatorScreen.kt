package com.mahaswarna.feature.calculator.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.ScrollableTabRow
import androidx.compose.material3.Tab
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.mahaswarna.core.util.InrFormatter
import com.mahaswarna.feature.calculator.domain.CalculatorMode

private val MODES = listOf(
    CalculatorMode.WeightToPrice,
    CalculatorMode.PriceToWeight,
    CalculatorMode.MakingCharges,
)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CalculatorScreen(
    onNavigateBack: () -> Unit,
    goldRatePerGram: Double,
    silverRatePerGram: Double,
    viewModel: CalculatorViewModel = hiltViewModel(),
) {
    // Push live rates from parent into ViewModel.
    LaunchedEffect(goldRatePerGram, silverRatePerGram) {
        viewModel.updateRates(goldRatePerGram, silverRatePerGram)
    }

    val selectedMetal by viewModel.selectedMetal.collectAsState()
    val mode by viewModel.mode.collectAsState()
    val weightInput by viewModel.weightInput.collectAsState()
    val priceInput by viewModel.priceInput.collectAsState()
    val makingCharges by viewModel.makingChargesPercent.collectAsState()
    val result by viewModel.result.collectAsState()

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Calculator") },
                navigationIcon = {
                    IconButton(onClick = onNavigateBack) {
                        Icon(Icons.Default.ArrowBack, contentDescription = "Back")
                    }
                },
                actions = {
                    IconButton(onClick = { viewModel.clearInputs() }) {
                        Icon(Icons.Default.Refresh, contentDescription = "Clear")
                    }
                },
            )
        }
    ) { innerPadding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(innerPadding)
                .verticalScroll(rememberScrollState())
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            // ── Metal selector ─────────────────────────────────────────────
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                listOf("gold", "silver").forEach { metal ->
                    FilterChip(
                        selected = selectedMetal == metal,
                        onClick = { viewModel.selectMetal(metal) },
                        label = {
                            val rate = if (metal == "gold") goldRatePerGram else silverRatePerGram
                            Text("${metal.replaceFirstChar { it.uppercase() }} · ${InrFormatter.formatRateShort(rate)}/g")
                        }
                    )
                }
            }

            // ── Mode tabs ──────────────────────────────────────────────────
            ScrollableTabRow(
                selectedTabIndex = MODES.indexOf(mode),
                edgePadding = 0.dp,
            ) {
                MODES.forEachIndexed { index, m ->
                    Tab(
                        selected = mode == m,
                        onClick = { viewModel.selectMode(m) },
                        text = { Text(m.displayName(), style = MaterialTheme.typography.labelMedium) },
                    )
                }
            }

            // ── Inputs ─────────────────────────────────────────────────────
            when (mode) {
                CalculatorMode.WeightToPrice -> {
                    OutlinedTextField(
                        value = weightInput,
                        onValueChange = viewModel::onWeightInputChange,
                        label = { Text("Weight (grams)") },
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
                CalculatorMode.PriceToWeight -> {
                    OutlinedTextField(
                        value = priceInput,
                        onValueChange = viewModel::onPriceInputChange,
                        label = { Text("Budget (₹)") },
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
                CalculatorMode.MakingCharges -> {
                    OutlinedTextField(
                        value = weightInput,
                        onValueChange = viewModel::onWeightInputChange,
                        label = { Text("Weight (grams)") },
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                    )
                    OutlinedTextField(
                        value = makingCharges,
                        onValueChange = viewModel::onMakingChargesChange,
                        label = { Text("Making charges (%)") },
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                        placeholder = { Text("e.g. 12") },
                    )
                }
            }

            // ── Result card ────────────────────────────────────────────────
            result.errorMessage?.let { msg ->
                Text(
                    text = msg,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.error,
                )
            }

            if (result.metalValue != null || result.weightGrams != null) {
                Card(
                    colors = CardDefaults.cardColors(
                        containerColor = MaterialTheme.colorScheme.primaryContainer
                    ),
                    modifier = Modifier.fillMaxWidth(),
                ) {
                    Column(
                        modifier = Modifier.padding(16.dp),
                        verticalArrangement = Arrangement.spacedBy(8.dp),
                    ) {
                        Text("Result", style = MaterialTheme.typography.labelMedium)
                        HorizontalDivider()

                        when (mode) {
                            CalculatorMode.WeightToPrice,
                            CalculatorMode.MakingCharges -> {
                                result.metalValue?.let { mv ->
                                    ResultRow("Metal value", InrFormatter.formatPrice(mv))
                                }
                                result.makingChargesValue?.let { mc ->
                                    ResultRow("Making charges", InrFormatter.formatPrice(mc))
                                }
                                result.totalValue?.let { total ->
                                    if (result.makingChargesValue != null) {
                                        HorizontalDivider()
                                        ResultRow(
                                            "Total",
                                            InrFormatter.formatPrice(total),
                                            highlighted = true,
                                        )
                                    } else {
                                        ResultRow("Total", InrFormatter.formatPrice(total), highlighted = true)
                                    }
                                }
                            }
                            CalculatorMode.PriceToWeight -> {
                                result.weightGrams?.let { w ->
                                    ResultRow("Weight", InrFormatter.formatWeight(w), highlighted = true)
                                }
                            }
                        }
                    }
                }
            }

            Spacer(Modifier.height(8.dp))
        }
    }
}

@Composable
private fun ResultRow(
    label: String,
    value: String,
    highlighted: Boolean = false,
) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            text = label,
            style = if (highlighted) MaterialTheme.typography.titleSmall
                    else MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onPrimaryContainer,
        )
        Text(
            text = value,
            style = if (highlighted) MaterialTheme.typography.titleMedium
                    else MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onPrimaryContainer,
        )
    }
}
