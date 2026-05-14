package com.mahaswarna.feature.marketplace.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
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
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavController

/**
 * Invoice generation screen.
 *
 * FIX: Original used `navController.currentBackStackEntry?.arguments?.getDouble("goldRate")`
 * which returns 0.0 for a missing key rather than null. With Compose Navigation typed routes
 * (serialized to bundle), nullable Double args must be read as `getString` and parsed, or
 * the ViewModel reads them from [SavedStateHandle] which is the correct pattern.
 *
 * [BillPrintViewModel] already holds the goldRate/silverRate via SavedStateHandle.
 * The Screen reads them from the VM, not directly from nav args.
 *
 * goldRate/silverRate from the nav args are passed so the invoice captures the rate snapshot
 * the user saw — not a re-fetch. If null (navigated from Diary, no rate context), the
 * backend uses the live rate from the Pricing service.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun BillPrintScreen(
    navController: NavController,
    viewModel: BillPrintViewModel = hiltViewModel(),
) {
    val uiState  by viewModel.uiState.collectAsStateWithLifecycle()
    val goldRate  = viewModel.goldRate    // read from SavedStateHandle in VM
    val silverRate = viewModel.silverRate
    val context  = LocalContext.current

    var customerName  by remember { mutableStateOf("") }
    var customerPhone by remember { mutableStateOf("") }
    var paymentMode   by remember { mutableStateOf("cash") }
    var notes         by remember { mutableStateOf("") }
    val lineItems     = remember { mutableStateListOf(LineItemForm()) }

    // Trigger share sheet when PDF is ready
    LaunchedEffect(uiState) {
        if (uiState is BillPrintUiState.PdfReady) {
            context.startActivity(
                android.content.Intent.createChooser(
                    (uiState as BillPrintUiState.PdfReady).shareIntent,
                    "Share Invoice",
                )
            )
            viewModel.reset()
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Generate Invoice") },
                navigationIcon = {
                    IconButton(onClick = { navController.navigateUp() }) {
                        Icon(Icons.Default.ArrowBack, contentDescription = "Back")
                    }
                },
            )
        },
    ) { innerPadding ->
        when (val state = uiState) {

            is BillPrintUiState.GeneratingPdf -> Box(
                modifier         = Modifier.fillMaxSize().padding(innerPadding),
                contentAlignment = Alignment.Center,
            ) { CircularProgressIndicator() }

            is BillPrintUiState.NoShopRegistered -> Box(
                modifier         = Modifier.fillMaxSize().padding(innerPadding),
                contentAlignment = Alignment.Center,
            ) {
                Column(
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(12.dp),
                    modifier = Modifier.padding(32.dp),
                ) {
                    Text("No shop registered. Register your shop first.", textAlign = TextAlign.Center)
                    Button(onClick = { navController.navigate(com.mahaswarna.navigation.Route.RegisterShop) }) {
                        Text("Register Shop")
                    }
                }
            }

            is BillPrintUiState.QuotaExceeded -> Box(
                modifier         = Modifier.fillMaxSize().padding(innerPadding),
                contentAlignment = Alignment.Center,
            ) {
                Column(
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(12.dp),
                    modifier = Modifier.padding(32.dp),
                ) {
                    Text(
                        "Daily invoice limit reached (60/day).\nTry again tomorrow.",
                        textAlign = TextAlign.Center,
                    )
                    TextButton(onClick = { navController.navigateUp() }) { Text("Back") }
                }
            }

            else -> {
                // Form — shown for Idle and Error states
                Column(
                    modifier = Modifier
                        .fillMaxSize()
                        .padding(innerPadding)
                        .padding(horizontal = 16.dp)
                        .verticalScroll(rememberScrollState()),
                    verticalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    if (state is BillPrintUiState.Error) {
                        Text(
                            text     = state.message,
                            color    = MaterialTheme.colorScheme.error,
                            style    = MaterialTheme.typography.bodySmall,
                            modifier = Modifier.padding(top = 8.dp),
                        )
                    }

                    Text("Customer", style = MaterialTheme.typography.titleMedium)

                    OutlinedTextField(
                        value         = customerName,
                        onValueChange = { customerName = it },
                        label         = { Text("Customer Name *") },
                        modifier      = Modifier.fillMaxWidth(),
                        singleLine    = true,
                    )
                    OutlinedTextField(
                        value           = customerPhone,
                        onValueChange   = { customerPhone = it },
                        label           = { Text("Phone") },
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Phone),
                        modifier        = Modifier.fillMaxWidth(),
                        singleLine      = true,
                    )

                    Text("Items", style = MaterialTheme.typography.titleMedium)

                    lineItems.forEachIndexed { index, item ->
                        LineItemRow(
                            item      = item,
                            canDelete = lineItems.size > 1,
                            onUpdate  = { lineItems[index] = it },
                            onDelete  = { lineItems.removeAt(index) },
                        )
                    }

                    TextButton(onClick = { lineItems.add(LineItemForm()) }, modifier = Modifier.fillMaxWidth()) {
                        Icon(Icons.Default.Add, contentDescription = null)
                        Text("Add Item")
                    }

                    OutlinedTextField(
                        value         = notes,
                        onValueChange = { notes = it },
                        label         = { Text("Notes (optional)") },
                        modifier      = Modifier.fillMaxWidth(),
                        maxLines      = 2,
                    )

                    if (goldRate != null && goldRate > 0.0) {
                        Text(
                            text  = "Rate snapshot: Gold ${InrFormatter.formatRateShort(goldRate)}/g",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }

                    Spacer(modifier = Modifier.height(4.dp))

                    Button(
                        onClick = {
                            viewModel.generateInvoice(
                                customerName  = customerName,
                                customerPhone = customerPhone.takeIf { it.isNotBlank() },
                                lineItems     = lineItems.toList(),
                                paymentMode   = paymentMode,
                                notes         = notes.takeIf { it.isNotBlank() },
                                goldRate      = goldRate,
                                silverRate    = silverRate,
                            )
                        },
                        enabled  = customerName.isNotBlank(),
                        modifier = Modifier.fillMaxWidth().padding(bottom = 24.dp),
                    ) {
                        Text("Generate & Share PDF")
                    }
                }
            }
        }
    }
}

@Composable
private fun LineItemRow(
    item: LineItemForm,
    canDelete: Boolean,
    onUpdate: (LineItemForm) -> Unit,
    onDelete: () -> Unit,
) {
    Column(modifier = Modifier.fillMaxWidth()) {
        Row(
            modifier          = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalAlignment = Alignment.Top,
        ) {
            OutlinedTextField(
                value         = item.description,
                onValueChange = { onUpdate(item.copy(description = it)) },
                label         = { Text("Item") },
                placeholder   = { Text("e.g. Gold Ring") },
                modifier      = Modifier.weight(1f),
                singleLine    = true,
            )
            if (canDelete) {
                IconButton(onClick = onDelete, modifier = Modifier.padding(top = 8.dp)) {
                    Icon(Icons.Default.Delete, contentDescription = "Remove item")
                }
            }
        }
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            OutlinedTextField(
                value           = item.weightGrams,
                onValueChange   = { onUpdate(item.copy(weightGrams = it)) },
                label           = { Text("Weight (g)") },
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                modifier        = Modifier.weight(1f),
                singleLine      = true,
            )
            OutlinedTextField(
                value           = item.makingCharge,
                onValueChange   = { onUpdate(item.copy(makingCharge = it)) },
                label           = { Text("Making (₹)") },
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                modifier        = Modifier.weight(1f),
                singleLine      = true,
            )
        }
    }
}
