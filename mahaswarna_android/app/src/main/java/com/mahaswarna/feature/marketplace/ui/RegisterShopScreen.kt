package com.mahaswarna.feature.marketplace.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.OutlinedTextField
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
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavController
import com.mahaswarna.feature.marketplace.domain.RegisterShopInput

/**
 * Shop registration form.
 * On success → pop back to ShopListScreen which will now show HasShop state.
 * On NotPremium → navigate to Paywall.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun RegisterShopScreen(
    navController: NavController,
    viewModel: ShopViewModel = hiltViewModel(),
) {
    val events by viewModel.events.collectAsStateWithLifecycle()
    val snackbarHostState = remember { SnackbarHostState() }

    var name by remember { mutableStateOf("") }
    var address by remember { mutableStateOf("") }
    var gst by remember { mutableStateOf("") }
    var phone by remember { mutableStateOf("") }
    var nameError by remember { mutableStateOf<String?>(null) }
    var gstError by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(events) {
        when (val e = events) {
            is ShopEvent.RegistrationComplete -> navController.navigateUp()
            is ShopEvent.ShowError -> snackbarHostState.showSnackbar(e.message)
            else -> Unit
        }
        if (events != null) viewModel.eventConsumed()
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Register Shop") },
                navigationIcon = {
                    IconButton(onClick = { navController.navigateUp() }) {
                        Icon(Icons.Default.ArrowBack, contentDescription = "Back")
                    }
                },
            )
        },
        snackbarHost = { SnackbarHost(snackbarHostState) },
    ) { innerPadding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(innerPadding)
                .padding(horizontal = 24.dp)
                .verticalScroll(rememberScrollState()),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            OutlinedTextField(
                value         = name,
                onValueChange = { name = it; nameError = null },
                label         = { Text("Shop Name *") },
                isError       = nameError != null,
                supportingText = nameError?.let { { Text(it) } },
                keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Words),
                modifier      = Modifier.fillMaxWidth(),
                singleLine    = true,
            )

            OutlinedTextField(
                value         = address,
                onValueChange = { address = it },
                label         = { Text("Address *") },
                modifier      = Modifier.fillMaxWidth(),
                maxLines      = 3,
            )

            OutlinedTextField(
                value         = phone,
                onValueChange = { phone = it },
                label         = { Text("Phone") },
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Phone),
                modifier      = Modifier.fillMaxWidth(),
                singleLine    = true,
            )

            OutlinedTextField(
                value         = gst,
                onValueChange = { gst = it.uppercase(); gstError = null },
                label         = { Text("GSTIN (15 characters)") },
                isError       = gstError != null,
                supportingText = gstError?.let { { Text(it) } },
                keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Characters),
                modifier      = Modifier.fillMaxWidth(),
                singleLine    = true,
                placeholder   = { Text("e.g. 27AABCU9603R1ZM") },
            )

            Button(
                onClick = {
                    nameError = if (name.isBlank()) "Shop name is required" else null
                    if (gst.isNotBlank() && gst.length != 15) {
                        gstError = "GSTIN must be 15 characters"
                    }
                    if (nameError != null || gstError != null) return@Button
                    viewModel.registerShop(
                        RegisterShopInput(
                            name      = name.trim(),
                            address   = address.trim(),
                            gstNumber = gst.trim(),
                            phone     = phone.trim(),
                        )
                    )
                },
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 8.dp),
            ) {
                Text("Register")
            }
        }
    }
}
