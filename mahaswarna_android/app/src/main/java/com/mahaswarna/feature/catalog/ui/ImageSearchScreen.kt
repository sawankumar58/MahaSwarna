package com.mahaswarna.feature.catalog.ui

import android.net.Uri
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material.icons.filled.CameraAlt
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
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewModelScope
import androidx.navigation.NavController
import com.mahaswarna.feature.catalog.domain.Design
import com.mahaswarna.feature.catalog.domain.ImageSearchUseCase
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

// ── UI State ──────────────────────────────────────────────────────────────────

sealed class ImageSearchUiState {
    data object Idle : ImageSearchUiState()
    data object Loading : ImageSearchUiState()
    data class Results(val designs: List<Design>) : ImageSearchUiState()
    data class Error(val message: String) : ImageSearchUiState()
}

// ── ViewModel ──────────────────────────────────────────────────────────────────

@HiltViewModel
class ImageSearchViewModel @Inject constructor(
    private val imageSearchUseCase: ImageSearchUseCase,
) : ViewModel() {

    private val _uiState = MutableStateFlow<ImageSearchUiState>(ImageSearchUiState.Idle)
    val uiState: StateFlow<ImageSearchUiState> = _uiState.asStateFlow()

    fun search(imageBytes: ByteArray) {
        viewModelScope.launch {
            _uiState.value = ImageSearchUiState.Loading
            imageSearchUseCase(imageBytes)
                .onSuccess { _uiState.value = ImageSearchUiState.Results(it) }
                .onFailure { _uiState.value = ImageSearchUiState.Error(it.message ?: "Image search failed") }
        }
    }
}

// ── Screen ─────────────────────────────────────────────────────────────────────

/**
 * Image search screen — pick a photo to find visually similar jewellery designs.
 *
 * IMPORTANT: Only register this composable in AppNavGraph when
 * killSwitchImageSearch == false. See Route.kt comment.
 *
 * FIX: Removed the bogus `private fun collectAsStateWithLifecycle() = Unit` at the
 * bottom of the original file that shadowed the real lifecycle extension from
 * `androidx.lifecycle.compose`, causing compile errors.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ImageSearchScreen(
    navController: NavController,
    viewModel: ImageSearchViewModel = hiltViewModel(),
) {
    val context = LocalContext.current
    val scope   = rememberCoroutineScope()
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()

    val imagePicker = rememberLauncherForActivityResult(
        ActivityResultContracts.GetContent(),
    ) { uri: Uri? ->
        if (uri == null) return@rememberLauncherForActivityResult
        scope.launch {
            val bytes = context.contentResolver.openInputStream(uri)?.readBytes()
                ?: return@launch
            viewModel.search(bytes)
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Visual Search") },
                navigationIcon = {
                    IconButton(onClick = { navController.navigateUp() }) {
                        Icon(Icons.Default.ArrowBack, contentDescription = "Back")
                    }
                },
            )
        },
    ) { innerPadding ->
        Column(
            modifier            = Modifier.fillMaxSize().padding(innerPadding).padding(16.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            when (val state = uiState) {
                is ImageSearchUiState.Idle -> {
                    Spacer(modifier = Modifier.height(64.dp))
                    Icon(Icons.Default.CameraAlt, contentDescription = null, tint = MaterialTheme.colorScheme.outlineVariant)
                    Spacer(modifier = Modifier.height(16.dp))
                    Text("Pick a jewellery photo to find similar designs", style = MaterialTheme.typography.bodyMedium)
                    Spacer(modifier = Modifier.height(24.dp))
                    Button(onClick = { imagePicker.launch("image/*") }) { Text("Choose Photo") }
                }

                is ImageSearchUiState.Loading -> Box(
                    modifier = Modifier.fillMaxSize(),
                    contentAlignment = Alignment.Center,
                ) { CircularProgressIndicator() }

                is ImageSearchUiState.Error -> Box(
                    modifier = Modifier.fillMaxSize(),
                    contentAlignment = Alignment.Center,
                ) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Text(state.message)
                        Spacer(modifier = Modifier.height(12.dp))
                        Button(onClick = { imagePicker.launch("image/*") }) { Text("Try Again") }
                    }
                }

                is ImageSearchUiState.Results -> {
                    Button(onClick = { imagePicker.launch("image/*") }) { Text("Try Another Photo") }
                    Spacer(modifier = Modifier.height(12.dp))
                    LazyVerticalGrid(
                        columns               = GridCells.Fixed(2),
                        contentPadding        = PaddingValues(top = 8.dp),
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                        verticalArrangement   = Arrangement.spacedBy(8.dp),
                        modifier              = Modifier.fillMaxWidth(),
                    ) {
                        items(state.designs, key = { it.id }) { design ->
                            DesignCard(design = design, onClick = {})
                        }
                    }
                }
            }
        }
    }
}
