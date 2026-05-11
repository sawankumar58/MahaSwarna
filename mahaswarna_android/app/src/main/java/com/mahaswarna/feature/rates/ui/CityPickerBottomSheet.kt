package com.mahaswarna.feature.rates.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.mahaswarna.core.network.ApiConstants

/**
 * City picker bottom sheet.
 *
 * Source: ApiConstants.CITY_LIST (63-city compile-time constant — no network call).
 * Supports search/filter by display name.
 * Calls onCitySelected(cityId) on tap; onDismiss on backdrop tap or swipe.
 *
 * Used in:
 *   - OtpScreen (city capture before login)
 *   - RatesDashboardScreen (city change after login)
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CityPickerBottomSheet(
    currentCityId: String,
    onCitySelected: (String) -> Unit,
    onDismiss: () -> Unit,
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var searchQuery by remember { mutableStateOf("") }

    val filteredCities = remember(searchQuery) {
        if (searchQuery.isBlank()) ApiConstants.CITY_LIST
        else ApiConstants.CITY_LIST.filter {
            it.displayName.contains(searchQuery, ignoreCase = true)
        }
    }

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
    ) {
        Column(modifier = Modifier.fillMaxWidth()) {
            Text(
                text = "Select City",
                style = MaterialTheme.typography.titleLarge,
                modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
            )

            OutlinedTextField(
                value = searchQuery,
                onValueChange = { searchQuery = it },
                placeholder = { Text("Search cities…") },
                leadingIcon = { Icon(Icons.Default.Search, contentDescription = null) },
                singleLine = true,
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp, vertical = 4.dp),
            )

            Spacer(Modifier.height(8.dp))

            LazyColumn(modifier = Modifier.fillMaxWidth()) {
                items(filteredCities, key = { it.id }) { city ->
                    ListItem(
                        headlineContent = { Text(city.displayName) },
                        trailingContent = {
                            if (city.id == currentCityId) {
                                Icon(Icons.Default.Check, contentDescription = "Selected")
                            }
                        },
                        modifier = Modifier.clickable { onCitySelected(city.id) },
                    )
                    HorizontalDivider()
                }
            }

            // Safe area bottom padding for gesture navigation
            Spacer(Modifier.height(WindowInsets.navigationBars.asPaddingValues().calculateBottomPadding()))
        }
    }
}
