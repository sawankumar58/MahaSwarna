package com.mahaswarna.feature.catalog.ui

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewModelScope
import androidx.navigation.NavController
import coil.compose.AsyncImage
import com.mahaswarna.feature.catalog.data.CatalogRepository
import com.mahaswarna.feature.catalog.domain.Design
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

// ── ViewModel ──────────────────────────────────────────────────────────────────
// Dedicated ViewModel for the design detail screen. Fetches a single design
// by ID from CatalogRepository and exposes it as a sealed UiState flow.

sealed class DesignDetailUiState {
    data object Loading : DesignDetailUiState()
    data class Success(val design: Design) : DesignDetailUiState()
    data class Error(val message: String) : DesignDetailUiState()
}

@HiltViewModel
class DesignDetailViewModel @Inject constructor(
    savedStateHandle: SavedStateHandle,
    private val repository: CatalogRepository,
) : ViewModel() {

    private val designId: String = checkNotNull(savedStateHandle["designId"])

    private val _uiState = MutableStateFlow<DesignDetailUiState>(DesignDetailUiState.Loading)
    val uiState: StateFlow<DesignDetailUiState> = _uiState.asStateFlow()

    init { load() }

    fun load() {
        viewModelScope.launch {
            _uiState.value = DesignDetailUiState.Loading
            runCatching { repository.getDesign(designId) }
                .onSuccess { _uiState.value = DesignDetailUiState.Success(it) }
                .onFailure { _uiState.value = DesignDetailUiState.Error(it.message ?: "Failed to load design") }
        }
    }
}

// ── Screen ─────────────────────────────────────────────────────────────────────

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun DesignDetailScreen(
    navController: NavController,
    viewModel: DesignDetailViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Text((uiState as? DesignDetailUiState.Success)?.design?.title ?: "Design")
                },
                navigationIcon = {
                    IconButton(onClick = { navController.navigateUp() }) {
                        Icon(Icons.Default.ArrowBack, contentDescription = "Back")
                    }
                },
            )
        },
    ) { innerPadding ->
        when (val state = uiState) {
            is DesignDetailUiState.Loading -> Box(
                modifier = Modifier.fillMaxSize().padding(innerPadding),
                contentAlignment = Alignment.Center,
            ) { CircularProgressIndicator() }

            is DesignDetailUiState.Error -> Box(
                modifier = Modifier.fillMaxSize().padding(innerPadding),
                contentAlignment = Alignment.Center,
            ) {
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    Text(state.message, style = MaterialTheme.typography.bodyMedium)
                    Spacer(modifier = Modifier.height(12.dp))
                    Button(onClick = viewModel::load) { Text("Retry") }
                }
            }

            is DesignDetailUiState.Success -> {
                val d = state.design
                Column(
                    modifier = Modifier
                        .fillMaxSize()
                        .padding(innerPadding)
                        .verticalScroll(rememberScrollState()),
                ) {
                    AsyncImage(
                        model              = d.imageUrl,
                        contentDescription = d.title,
                        contentScale       = ContentScale.Crop,
                        modifier           = Modifier.fillMaxWidth().height(280.dp),
                    )
                    Column(modifier = Modifier.padding(20.dp)) {
                        Text(d.title, style = MaterialTheme.typography.headlineSmall)
                        Spacer(modifier = Modifier.height(4.dp))
                        Text(
                            text  = d.metalType.replaceFirstChar { it.uppercase() } +
                                    if (d.weightGrams > 0) " · ${d.weightGrams}g" else "",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                        if (d.description.isNotBlank()) {
                            Spacer(modifier = Modifier.height(12.dp))
                            Text(d.description, style = MaterialTheme.typography.bodyLarge)
                        }
                        if (d.tags.isNotEmpty()) {
                            Spacer(modifier = Modifier.height(12.dp))
                            Text(
                                text  = d.tags.joinToString(" · ") { "#$it" },
                                style = MaterialTheme.typography.labelMedium,
                                color = MaterialTheme.colorScheme.primary,
                            )
                        }
                        Spacer(modifier = Modifier.height(8.dp))
                        Text(
                            text  = "${d.viewCount} views",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                }
            }
        }
    }
}
