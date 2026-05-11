package com.mahaswarna.feature.alerts.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp

/**
 * Bottom sheet for creating a price threshold alert.
 *
 * Users select:
 *  - Metal: Gold | Silver
 *  - Direction: Above | Below
 *  - Threshold: price per gram in INR (free-text, numeric)
 *
 * [defaultCityId] is the user's currently selected city (from PreferenceStore / HomeViewModel).
 * In Phase 1 the city is not user-selectable here; it is derived from the home screen context.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CreateAlertBottomSheet(
    defaultCityId: String,
    onDismiss: () -> Unit,
    onCreate: (cityId: String, metal: String, threshold: Double, direction: String) -> Unit,
) {
    val sheetState = rememberModalBottomSheetState(skipPartialExpansion = true)

    var selectedMetal by remember { mutableStateOf("gold") }
    var selectedDirection by remember { mutableStateOf("above") }
    var thresholdText by remember { mutableStateOf("") }
    var thresholdError by remember { mutableStateOf<String?>(null) }

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 24.dp)
                .navigationBarsPadding()
                .padding(bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            Text("New Price Alert", style = MaterialTheme.typography.titleLarge)

            // ── Metal selector ─────────────────────────────────────────────
            Text("Metal", style = MaterialTheme.typography.labelMedium)
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                listOf("gold", "silver").forEach { metal ->
                    FilterChip(
                        selected = selectedMetal == metal,
                        onClick = { selectedMetal = metal },
                        label = { Text(metal.replaceFirstChar { it.uppercase() }) },
                    )
                }
            }

            // ── Direction selector ─────────────────────────────────────────
            Text("Alert when price goes", style = MaterialTheme.typography.labelMedium)
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                listOf("above", "below").forEach { dir ->
                    FilterChip(
                        selected = selectedDirection == dir,
                        onClick = { selectedDirection = dir },
                        label = { Text(dir.replaceFirstChar { it.uppercase() }) },
                    )
                }
            }

            // ── Threshold input ────────────────────────────────────────────
            OutlinedTextField(
                value = thresholdText,
                onValueChange = { value ->
                    thresholdText = value
                    thresholdError = null
                },
                label = { Text("Price per gram (₹)") },
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                isError = thresholdError != null,
                supportingText = thresholdError?.let { { Text(it) } },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )

            Spacer(Modifier.height(4.dp))

            // ── Actions ────────────────────────────────────────────────────
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp, alignment = androidx.compose.ui.Alignment.End),
            ) {
                TextButton(onClick = onDismiss) { Text("Cancel") }

                Button(
                    onClick = {
                        val threshold = thresholdText.toDoubleOrNull()
                        when {
                            threshold == null || thresholdText.isBlank() -> {
                                thresholdError = "Enter a valid price"
                            }
                            threshold <= 0 -> {
                                thresholdError = "Price must be greater than 0"
                            }
                            threshold > 1_000_000 -> {
                                thresholdError = "Price seems too high — please check"
                            }
                            else -> {
                                onCreate(defaultCityId, selectedMetal, threshold, selectedDirection)
                            }
                        }
                    }
                ) {
                    Text("Create Alert")
                }
            }
        }
    }
}
