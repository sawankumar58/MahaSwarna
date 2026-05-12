package com.mahaswarna.feature.diary.ui

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Tab
import androidx.compose.material3.TabRow
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavController

/**
 * DiaryScreen — two-tab layout:
 *   Tab 0: Customers — local address book with net balance badges.
 *   Tab 1: Bills    — full bill history, searchable via FTS.
 *
 * FAB changes context per tab:
 *   Customers tab → add customer bottom sheet.
 *   Bills tab     → navigates to BillPrint (invoice generation).
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun DiaryScreen(
    navController: NavController,
    viewModel: DiaryViewModel = hiltViewModel(),
) {
    val events by viewModel.events.collectAsStateWithLifecycle()
    val snackbarHostState = remember { SnackbarHostState() }
    var selectedTab by remember { mutableIntStateOf(0) }
    var showAddCustomerSheet by remember { mutableStateOf(false) }

    // ── Event handling ─────────────────────────────────────────────────────
    LaunchedEffect(events) {
        when (val e = events) {
            is DiaryUiEvent.ShowError       -> snackbarHostState.showSnackbar(e.message)
            is DiaryUiEvent.CustomerSaved   -> snackbarHostState.showSnackbar("Customer saved")
            is DiaryUiEvent.LedgerEntrySaved -> snackbarHostState.showSnackbar("Entry added")
            is DiaryUiEvent.BillSaved       -> snackbarHostState.showSnackbar("Bill saved")
            null -> Unit
        }
        if (events != null) viewModel.eventConsumed()
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Diary") },
                navigationIcon = {
                    IconButton(onClick = { navController.navigateUp() }) {
                        Icon(Icons.Default.ArrowBack, contentDescription = "Back")
                    }
                },
            )
        },
        floatingActionButton = {
            FloatingActionButton(onClick = {
                if (selectedTab == 0) showAddCustomerSheet = true
                else navController.navigate(
                    com.mahaswarna.navigation.Route.BillPrint(
                        goldRate = null, silverRate = null, isStale = false,
                    )
                )
            }) {
                Icon(Icons.Default.Add, contentDescription = "Add")
            }
        },
        snackbarHost = { SnackbarHost(snackbarHostState) },
    ) { innerPadding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(innerPadding),
        ) {
            TabRow(selectedTabIndex = selectedTab, modifier = Modifier.fillMaxWidth()) {
                Tab(
                    selected = selectedTab == 0,
                    onClick  = { selectedTab = 0 },
                    text     = { Text("Customers") },
                )
                Tab(
                    selected = selectedTab == 1,
                    onClick  = { selectedTab = 1 },
                    text     = { Text("Bills") },
                )
            }

            when (selectedTab) {
                0 -> CustomersTab(
                    navController = navController,
                    viewModel     = viewModel,
                )
                1 -> LedgerTab(
                    navController = navController,
                    viewModel     = viewModel,
                )
            }
        }
    }

    // ── Add customer bottom sheet ──────────────────────────────────────────
    if (showAddCustomerSheet) {
        AddCustomerBottomSheet(
            onDismiss = { showAddCustomerSheet = false },
            onSave = { name, phone, address, gst, notes ->
                viewModel.saveCustomer(
                    name      = name,
                    phone     = phone,
                    address   = address,
                    gstNumber = gst,
                    notes     = notes,
                )
                showAddCustomerSheet = false
            },
        )
    }
}
