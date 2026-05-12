package com.mahaswarna.feature.diary.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Person
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavController
import com.mahaswarna.core.util.InrFormatter
import com.mahaswarna.navigation.Route

@Composable
fun CustomersTab(
    navController: NavController,
    viewModel: DiaryViewModel,
) {
    val query by viewModel.searchQuery.collectAsStateWithLifecycle()
    val customers by viewModel.customersWithBalance.collectAsStateWithLifecycle()

    Column(modifier = Modifier.fillMaxSize()) {
        OutlinedTextField(
            value         = query,
            onValueChange = viewModel::onSearchQueryChange,
            placeholder   = { Text("Search customers…") },
            leadingIcon   = { Icon(Icons.Default.Search, contentDescription = null) },
            singleLine    = true,
            modifier      = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 8.dp),
        )

        if (customers.isEmpty()) {
            Column(
                modifier = Modifier.fillMaxSize(),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.Center,
            ) {
                Icon(
                    Icons.Default.Person,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.outlineVariant,
                )
                Text(
                    text      = "No customers yet.\nTap + to add your first customer.",
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
                items(customers, key = { it.customer.id }) { item ->
                    CustomerCard(
                        item    = item,
                        onClick = { navController.navigate(Route.CustomerDetail(item.customer.id)) },
                    )
                }
            }
        }
    }
}

@Composable
private fun CustomerCard(item: CustomerWithBalance, onClick: () -> Unit) {
    Card(
        modifier  = Modifier.fillMaxWidth().clickable(onClick = onClick),
        elevation = CardDefaults.cardElevation(defaultElevation = 1.dp),
    ) {
        Row(
            modifier          = Modifier.fillMaxWidth().padding(16.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text(item.customer.name, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
                if (item.customer.phone.isNotBlank()) {
                    Text(item.customer.phone, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
            Spacer(modifier = Modifier.width(12.dp))
            BalanceBadge(balance = item.netBalanceInr)
        }
    }
}

@Composable
private fun BalanceBadge(balance: Double) {
    // FIX: InrFormatter has no .format() — use .formatPrice()
    val (text, color) = when {
        balance > 0.01  -> InrFormatter.formatPrice(balance)  to Color(0xFFB71C1C)
        balance < -0.01 -> InrFormatter.formatPrice(-balance) to Color(0xFF1B5E20)
        else            -> "Settled"                          to MaterialTheme.colorScheme.onSurfaceVariant
    }
    Text(text, style = MaterialTheme.typography.labelLarge, color = color, fontWeight = FontWeight.Medium)
}
