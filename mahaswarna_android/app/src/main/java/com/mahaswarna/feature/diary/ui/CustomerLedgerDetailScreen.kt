package com.mahaswarna.feature.diary.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewModelScope
import androidx.navigation.NavController
import com.mahaswarna.feature.diary.domain.AddLedgerEntryUseCase
import com.mahaswarna.feature.diary.domain.GetCustomerLedgerUseCase
import com.mahaswarna.feature.diary.domain.LedgerEntry
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.flatMapLatest
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import javax.inject.Inject

// ── ViewModel ──────────────────────────────────────────────────────────────────
//
// FIX: Previous version defined a conflicting flatMapLatest extension function
// in the same file that shadowed kotlinx.coroutines.flow.flatMapLatest, causing
// compile errors. ViewModel is now clean — standard API only.

@OptIn(ExperimentalCoroutinesApi::class)
@HiltViewModel
class CustomerLedgerViewModel @Inject constructor(
    private val getCustomerLedger: GetCustomerLedgerUseCase,
    private val addLedgerEntryUseCase: AddLedgerEntryUseCase,
) : ViewModel() {

    private val _customerId = MutableStateFlow("")

    fun setCustomerId(id: String) {
        if (_customerId.value != id) _customerId.value = id
    }

    val entries: StateFlow<List<LedgerEntry>> = _customerId
        .flatMapLatest { cid ->
            if (cid.isBlank()) flowOf(emptyList()) else getCustomerLedger(cid)
        }
        .stateIn(
            scope        = viewModelScope,
            started      = SharingStarted.WhileSubscribed(5_000),
            initialValue = emptyList(),
        )

    fun addEntry(customerId: String, type: String, amountInr: Double, description: String) {
        // FIX: missing `import kotlinx.coroutines.launch` in original — now explicit above
        viewModelScope.launch {
            addLedgerEntryUseCase(
                AddLedgerEntryUseCase.Input(
                    customerId  = customerId,
                    type        = type,
                    amountInr   = amountInr,
                    description = description,
                )
            )
        }
    }
}

// ── Screen ─────────────────────────────────────────────────────────────────────

/**
 * Shows a single customer's full ledger: credit/debit entries and running balance.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CustomerLedgerDetailScreen(
    navController: NavController,
    viewModel: CustomerLedgerViewModel = hiltViewModel(),
) {
    val customerId = navController.currentBackStackEntry
        ?.arguments?.getString("customerId") ?: return

    LaunchedEffect(customerId) { viewModel.setCustomerId(customerId) }

    val entries by viewModel.entries.collectAsStateWithLifecycle()
    val netBalance = entries.fold(0.0) { acc, e ->
        if (e.type == "credit") acc + e.amountInr else acc - e.amountInr
    }
    val snackbarHostState = remember { SnackbarHostState() }
    var showAddSheet by remember { mutableStateOf(false) }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Ledger") },
                navigationIcon = {
                    IconButton(onClick = { navController.navigateUp() }) {
                        Icon(Icons.Default.ArrowBack, contentDescription = "Back")
                    }
                },
            )
        },
        floatingActionButton = {
            FloatingActionButton(onClick = { showAddSheet = true }) {
                Icon(Icons.Default.Add, contentDescription = "Add entry")
            }
        },
        snackbarHost = { SnackbarHost(snackbarHostState) },
    ) { padding ->
        Column(modifier = Modifier.fillMaxSize().padding(padding)) {
            BalanceSummaryCard(netBalance = netBalance)
            HorizontalDivider()
            LazyColumn(
                contentPadding      = PaddingValues(16.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                items(entries, key = { it.id }) { entry ->
                    LedgerEntryRow(entry = entry)
                }
            }
        }
    }

    if (showAddSheet) {
        AddLedgerEntryBottomSheet(
            customerId = customerId,
            onDismiss  = { showAddSheet = false },
            onSave     = { type, amount, desc ->
                viewModel.addEntry(customerId, type, amount, desc)
                showAddSheet = false
            },
        )
    }
}

@Composable
private fun BalanceSummaryCard(netBalance: Double) {
    val (label, color) = when {
        netBalance > 0.01  -> "Customer owes ₹${String.format("%.2f", netBalance)}" to Color(0xFFB71C1C)
        netBalance < -0.01 -> "Shop owes ₹${String.format("%.2f", -netBalance)}"    to Color(0xFF1B5E20)
        else               -> "Account settled"                                       to MaterialTheme.colorScheme.onSurfaceVariant
    }
    Card(
        modifier = Modifier.fillMaxWidth().padding(16.dp),
        colors   = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant),
    ) {
        Text(label, color = color, fontWeight = FontWeight.SemiBold, style = MaterialTheme.typography.titleMedium, modifier = Modifier.padding(16.dp))
    }
}

@Composable
private fun LedgerEntryRow(entry: LedgerEntry) {
    val dateFmt = remember { SimpleDateFormat("dd MMM yy", Locale("en", "IN")) }
    Row(
        modifier              = Modifier.fillMaxWidth().padding(vertical = 4.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment     = Alignment.CenterVertically,
    ) {
        Column(modifier = Modifier.weight(1f)) {
            Text(
                entry.description.ifBlank { if (entry.type == "credit") "Credit" else "Debit" },
                style = MaterialTheme.typography.bodyMedium,
            )
            Text(dateFmt.format(Date(entry.createdAt)), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        Text(
            text       = if (entry.type == "credit") "+₹${String.format("%.2f", entry.amountInr)}"
                         else "-₹${String.format("%.2f", entry.amountInr)}",
            color      = if (entry.type == "credit") Color(0xFF1B5E20) else Color(0xFFB71C1C),
            style      = MaterialTheme.typography.bodyMedium,
            fontWeight = FontWeight.Medium,
        )
    }
}
