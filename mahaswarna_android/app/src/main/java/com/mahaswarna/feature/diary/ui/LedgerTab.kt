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
import androidx.compose.material.icons.filled.Receipt
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavController
import com.mahaswarna.core.util.InrFormatter
import com.mahaswarna.feature.diary.domain.DiaryBill
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

@Composable
fun LedgerTab(
    navController: NavController,
    viewModel: DiaryViewModel,
) {
    val query by viewModel.searchQuery.collectAsStateWithLifecycle()
    val bills by viewModel.allBills.collectAsStateWithLifecycle()

    Column(modifier = Modifier.fillMaxSize()) {
        OutlinedTextField(
            value         = query,
            onValueChange = viewModel::onSearchQueryChange,
            placeholder   = { Text("Search bills…") },
            leadingIcon   = { Icon(Icons.Default.Search, contentDescription = null) },
            singleLine    = true,
            modifier      = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 8.dp),
        )

        if (bills.isEmpty()) {
            Column(
                modifier = Modifier.fillMaxSize(),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.Center,
            ) {
                Icon(Icons.Default.Receipt, contentDescription = null, tint = MaterialTheme.colorScheme.outlineVariant)
                Text(
                    text      = "No bills yet.\nGenerate your first invoice from Rates or Calculator.",
                    style     = MaterialTheme.typography.bodyMedium,
                    color     = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier  = Modifier.padding(top = 8.dp),
                    textAlign = TextAlign.Center,
                )
            }
        } else {
            LazyColumn(
                contentPadding      = PaddingValues(16.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                items(bills, key = { it.id }) { bill ->
                    BillCard(bill = bill)
                }
            }
        }
    }
}

@Composable
private fun BillCard(bill: DiaryBill) {
    val dateFmt = remember { SimpleDateFormat("dd MMM yy", Locale("en", "IN")) }

    Card(
        modifier  = Modifier.fillMaxWidth(),
        elevation = CardDefaults.cardElevation(defaultElevation = 1.dp),
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(
                modifier              = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment     = Alignment.Top,
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Text(bill.customerName, style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold)
                    Text(
                        "${bill.metalType.replaceFirstChar { it.uppercase() }} · ${bill.totalWeightG}g",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
                Column(horizontalAlignment = Alignment.End) {
                    // FIX: InrFormatter.format() does not exist — use formatPrice()
                    Text(InrFormatter.formatPrice(bill.totalInr), style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.Bold)
                    Text(dateFmt.format(Date(bill.createdAt)), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
            if (bill.paymentMode != "cash") {
                Text(
                    bill.paymentMode.replaceFirstChar { it.uppercase() },
                    style    = MaterialTheme.typography.labelSmall,
                    color    = MaterialTheme.colorScheme.primary,
                    modifier = Modifier.padding(top = 4.dp),
                )
            }
        }
    }
}
