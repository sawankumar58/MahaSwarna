package com.mahaswarna.feature.catalog.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material.icons.filled.CameraAlt
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.Card
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
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavController
import coil.compose.AsyncImage
import com.mahaswarna.feature.catalog.domain.Design
import com.mahaswarna.navigation.Route
import kotlinx.coroutines.FlowPreview
import kotlinx.coroutines.flow.debounce

/**
 * CatalogScreen — browsable design catalog with text search and infinite scroll.
 *
 * Kill-switch: [CatalogUiState.Disabled] is shown when killSwitchCatalog == true.
 * The tab itself should be hidden by HomeScreen when killSwitchCatalog is true,
 * so this state is only a safety net for deep-links.
 *
 * Image search icon is ONLY shown when killSwitchImageSearch == false.
 */
@OptIn(ExperimentalMaterial3Api::class, FlowPreview::class)
@Composable
fun CatalogScreen(
    navController: NavController,
    viewModel: CatalogViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()
    val query by viewModel.query.collectAsStateWithLifecycle()

    // Debounced search: fire after 400ms of idle typing
    LaunchedEffect(Unit) {
        snapshotFlow { query }
            .debounce(400)
            .collect { q -> viewModel.search(q) }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Catalog") },
                navigationIcon = {
                    IconButton(onClick = { navController.navigateUp() }) {
                        Icon(Icons.Default.ArrowBack, contentDescription = "Back")
                    }
                },
                actions = {
                    // Image search icon — only when kill-switch is disabled
                    if (viewModel.imageSearchEnabled) {
                        IconButton(onClick = {
                            // Route.ImageSearch is added to NavHost only when killSwitchImageSearch == false
                            // navController.navigate(Route.ImageSearch)
                        }) {
                            Icon(Icons.Default.CameraAlt, contentDescription = "Image search")
                        }
                    }
                },
            )
        },
    ) { innerPadding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(innerPadding),
        ) {
            OutlinedTextField(
                value         = query,
                onValueChange = viewModel::onQueryChange,
                placeholder   = { Text("Search designs, styles, metal…") },
                leadingIcon   = { Icon(Icons.Default.Search, contentDescription = null) },
                singleLine    = true,
                modifier      = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp, vertical = 8.dp),
            )

            when (val state = uiState) {
                is CatalogUiState.Loading -> {
                    Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                        CircularProgressIndicator()
                    }
                }

                is CatalogUiState.Disabled -> {
                    Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                        Text(
                            "Catalog is temporarily unavailable.",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                }

                is CatalogUiState.Error -> {
                    Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                        Column(horizontalAlignment = Alignment.CenterHorizontally) {
                            Text(state.message, style = MaterialTheme.typography.bodyMedium)
                            TextButton(onClick = viewModel::retry) { Text("Retry") }
                        }
                    }
                }

                is CatalogUiState.Success -> {
                    LazyVerticalGrid(
                        columns        = GridCells.Fixed(2),
                        contentPadding = PaddingValues(12.dp),
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                        verticalArrangement   = Arrangement.spacedBy(8.dp),
                    ) {
                        items(state.designs, key = { it.id }) { design ->
                            DesignCard(
                                design  = design,
                                onClick = { navController.navigate(Route.Catalog) }, // TODO: Route.DesignDetail(design.id) — add in next phase
                            )
                        }

                        // Load-more trigger
                        if (state.page < state.totalPages) {
                            item(span = { GridItemSpan(2) }) {
                                if (state.isLoadingMore) {
                                    Box(
                                        modifier = Modifier
                                            .fillMaxWidth()
                                            .padding(16.dp),
                                        contentAlignment = Alignment.Center,
                                    ) {
                                        CircularProgressIndicator()
                                    }
                                } else {
                                    LaunchedEffect(state.page) { viewModel.loadNextPage() }
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun DesignCard(
    design: Design,
    onClick: () -> Unit,
) {
    Card(modifier = Modifier.clickable(onClick = onClick)) {
        Column {
            AsyncImage(
                model             = design.imageUrl,
                contentDescription = design.title,
                contentScale      = ContentScale.Crop,
                modifier          = Modifier
                    .fillMaxWidth()
                    .height(140.dp),
            )
            Column(modifier = Modifier.padding(8.dp)) {
                Text(
                    text     = design.title,
                    style    = MaterialTheme.typography.labelLarge,
                    maxLines = 2,
                    overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis,
                )
                Text(
                    text  = design.metalType.replaceFirstChar { it.uppercase() },
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
    }
}
