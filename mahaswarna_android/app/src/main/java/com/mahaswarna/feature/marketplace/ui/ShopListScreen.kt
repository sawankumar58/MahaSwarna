package com.mahaswarna.feature.marketplace.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material.icons.filled.Edit
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
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
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavController
import coil.compose.AsyncImage
import com.mahaswarna.feature.marketplace.domain.Shop
import com.mahaswarna.navigation.Route

/**
 * Shop dashboard — shows the jeweller's registered shop or an onboarding CTA.
 * PREMIUM gate is enforced in [RegisterShopUseCase]; this screen is reachable by any user
 * but the register CTA is grayed out and shows a paywall prompt for FREE users.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ShopListScreen(
    navController: NavController,
    viewModel: ShopViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()
    val events by viewModel.events.collectAsStateWithLifecycle()
    val snackbarHostState = remember { SnackbarHostState() }

    LaunchedEffect(events) {
        when (val e = events) {
            is ShopEvent.ShowError   -> snackbarHostState.showSnackbar(e.message)
            is ShopEvent.ShowSuccess -> snackbarHostState.showSnackbar(e.message)
            is ShopEvent.BannerUploaded -> snackbarHostState.showSnackbar("Banner uploaded. Moderation in progress…")
            else -> Unit
        }
        if (events != null) viewModel.eventConsumed()
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("My Shop") },
                navigationIcon = {
                    IconButton(onClick = { navController.navigateUp() }) {
                        Icon(Icons.Default.ArrowBack, contentDescription = "Back")
                    }
                },
            )
        },
        snackbarHost = { SnackbarHost(snackbarHostState) },
    ) { innerPadding ->
        Box(
            modifier = Modifier
                .fillMaxSize()
                .padding(innerPadding),
            contentAlignment = Alignment.Center,
        ) {
            when (val state = uiState) {
                is ShopUiState.Loading -> CircularProgressIndicator()

                is ShopUiState.Error -> Column(
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    Text(state.message, style = MaterialTheme.typography.bodyMedium)
                    Button(onClick = viewModel::loadShops) { Text("Retry") }
                }

                is ShopUiState.NoShop -> Column(
                    modifier = Modifier.padding(32.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(16.dp),
                ) {
                    Text(
                        "Register your shop on MahaSwarna Marketplace",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.SemiBold,
                        textAlign = androidx.compose.ui.text.style.TextAlign.Center,
                    )
                    Text(
                        "List your store, generate invoices, and reach customers across India.",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        textAlign = androidx.compose.ui.text.style.TextAlign.Center,
                    )
                    Button(onClick = { navController.navigate(Route.RegisterShop) }) {
                        Text("Register Shop (Premium)")
                    }
                }

                is ShopUiState.HasShop -> ShopProfile(
                    shop = state.shop,
                    onEdit = { navController.navigate(Route.EditShop(state.shop.id)) },
                    onBanner = { navController.navigate(Route.BannerPicker(state.shop.id)) },
                    onGenerateInvoice = {
                        navController.navigate(
                            Route.BillPrint(goldRate = null, silverRate = null, isStale = false)
                        )
                    },
                )
            }
        }
    }
}

@Composable
private fun ShopProfile(
    shop: Shop,
    onEdit: () -> Unit,
    onBanner: () -> Unit,
    onGenerateInvoice: () -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        // Banner
        if (shop.bannerUrl != null) {
            AsyncImage(
                model              = shop.bannerUrl,
                contentDescription = "Shop banner",
                contentScale       = ContentScale.Crop,
                modifier           = Modifier
                    .fillMaxWidth()
                    .height(160.dp),
            )
        }

        Text(shop.name, style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold)
        if (shop.address.isNotBlank()) Text(shop.address, style = MaterialTheme.typography.bodyMedium)
        if (shop.phone.isNotBlank()) Text(shop.phone, style = MaterialTheme.typography.bodyMedium)
        if (shop.gstNumber.isNotBlank()) Text("GSTIN: ${shop.gstNumber}", style = MaterialTheme.typography.bodySmall)

        Spacer(modifier = Modifier.height(8.dp))

        Button(onClick = onBanner, modifier = Modifier.fillMaxWidth()) {
            Text(if (shop.bannerUrl != null) "Change Banner" else "Upload Banner")
        }
        Button(onClick = onGenerateInvoice, modifier = Modifier.fillMaxWidth()) {
            Text("Generate Invoice")
        }
        Button(onClick = onEdit, modifier = Modifier.fillMaxWidth()) {
            Icon(Icons.Default.Edit, contentDescription = null)
            Text("Edit Shop", modifier = Modifier.padding(start = 8.dp))
        }
    }
}
