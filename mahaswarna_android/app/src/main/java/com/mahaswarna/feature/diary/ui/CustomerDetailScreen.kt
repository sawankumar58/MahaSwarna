package com.mahaswarna.feature.diary.ui

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AccountBalance
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.Edit
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavController
import com.mahaswarna.core.util.InrFormatter
import com.mahaswarna.navigation.Route

/**
 * Shows a customer's profile details (name, phone, address, GSTIN)
 * and provides navigation to their ledger and bill history.
 *
 * Destructive delete requires confirmation dialog.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CustomerDetailScreen(
    navController: NavController,
    viewModel: DiaryViewModel = hiltViewModel(),
) {
    val backStackEntry = navController.currentBackStackEntry
    val customerId = backStackEntry
        ?.arguments
        ?.getString("customerId") ?: return

    val customers by viewModel.customersWithBalance.collectAsStateWithLifecycle()
    val item = customers.firstOrNull { it.customer.id == customerId }
    var showDeleteDialog by remember { mutableStateOf(false) }

    LaunchedEffect(viewModel.events.collectAsStateWithLifecycle().value) {
        // Pop back if customer was deleted
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(item?.customer?.name ?: "Customer") },
                navigationIcon = {
                    IconButton(onClick = { navController.navigateUp() }) {
                        Icon(Icons.Default.ArrowBack, contentDescription = "Back")
                    }
                },
                actions = {
                    IconButton(onClick = {
                        navController.navigate(Route.CustomerLedgerDetail(customerId))
                    }) {
                        Icon(Icons.Default.AccountBalance, contentDescription = "Ledger")
                    }
                    IconButton(onClick = { showDeleteDialog = true }) {
                        Icon(Icons.Default.Delete, contentDescription = "Delete customer")
                    }
                },
            )
        },
    ) { innerPadding ->
        if (item == null) {
            Text("Loading…", modifier = Modifier.padding(innerPadding).padding(16.dp))
        } else {
            val customer = item.customer
            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(innerPadding)
                    .padding(24.dp),
            ) {
                DetailRow("Name",    customer.name)
                if (customer.phone.isNotBlank()) DetailRow("Phone",   customer.phone)
                if (customer.address.isNotBlank()) DetailRow("Address", customer.address)
                if (customer.gstNumber.isNotBlank()) DetailRow("GSTIN", customer.gstNumber)
                if (customer.notes.isNotBlank()) DetailRow("Notes",   customer.notes)
                Spacer(modifier = Modifier.height(24.dp))
                BalanceSectionHeader(balance = item.netBalanceInr)
            }
        }
    }

    if (showDeleteDialog) {
        AlertDialog(
            onDismissRequest = { showDeleteDialog = false },
            title = { Text("Delete customer?") },
            text  = { Text("This will delete all ledger entries for this customer. Linked bills remain.") },
            confirmButton = {
                TextButton(onClick = {
                    viewModel.deleteCustomer(customerId)
                    showDeleteDialog = false
                    navController.navigateUp()
                }) { Text("Delete", color = MaterialTheme.colorScheme.error) }
            },
            dismissButton = {
                TextButton(onClick = { showDeleteDialog = false }) { Text("Cancel") }
            },
        )
    }
}

@Composable
private fun DetailRow(label: String, value: String) {
    Column(modifier = Modifier.padding(bottom = 12.dp)) {
        Text(label, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text(value, style = MaterialTheme.typography.bodyLarge)
    }
}

@Composable
private fun BalanceSectionHeader(balance: Double) {
    val label = when {
        balance > 0.01  -> "Customer owes ${InrFormatter.formatPrice(balance)}"
        balance < -0.01 -> "Shop owes ${InrFormatter.formatPrice(-balance)}"
        else            -> "Account settled"
    }
    val color = when {
        balance > 0.01  -> androidx.compose.ui.graphics.Color(0xFFB71C1C)
        balance < -0.01 -> androidx.compose.ui.graphics.Color(0xFF1B5E20)
        else            -> MaterialTheme.colorScheme.onSurfaceVariant
    }
    Text(label, style = MaterialTheme.typography.titleMedium, color = color, fontWeight = FontWeight.SemiBold)
}
