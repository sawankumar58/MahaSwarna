package com.mahaswarna.feature.marketplace.ui

import android.net.Uri
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
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
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavController
import coil.compose.AsyncImage

/**
 * Banner picker and upload screen.
 * Flow: pick image → preview → upload (PUT to S3 presigned URL) → confirm with backend.
 * Backend runs async moderation; [bannerUrl] is populated only after moderation passes.
 *
 * Accepts only JPEG/PNG; server validates content-type on the S3 side.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ShopBannerScreen(
    navController: NavController,
    viewModel: ShopViewModel = hiltViewModel(),
) {
    val shopId = navController.currentBackStackEntry
        ?.arguments?.getString("shopId") ?: return

    val events by viewModel.events.collectAsStateWithLifecycle()
    val snackbarHostState = remember { SnackbarHostState() }
    val context = LocalContext.current
    var selectedUri by remember { mutableStateOf<Uri?>(null) }
    var uploading by remember { mutableStateOf(false) }

    LaunchedEffect(events) {
        when (val e = events) {
            is ShopEvent.BannerUploaded -> {
                snackbarHostState.showSnackbar("Banner submitted for review. It'll appear shortly.")
                uploading = false
                navController.navigateUp()
            }
            is ShopEvent.ShowError -> {
                snackbarHostState.showSnackbar(e.message)
                uploading = false
            }
            else -> Unit
        }
        if (events != null) viewModel.eventConsumed()
    }

    val imagePicker = rememberLauncherForActivityResult(
        ActivityResultContracts.GetContent()
    ) { uri: Uri? -> selectedUri = uri }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Shop Banner") },
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
                .padding(24.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            Text(
                "Upload a high-quality banner image for your shop.\n" +
                "Images are reviewed for content compliance before going live.",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                textAlign = TextAlign.Center,
            )

            if (selectedUri != null) {
                AsyncImage(
                    model              = selectedUri,
                    contentDescription = "Selected banner",
                    contentScale       = ContentScale.Crop,
                    modifier           = Modifier
                        .fillMaxWidth()
                        .height(180.dp),
                )
            } else {
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(180.dp),
                    contentAlignment = Alignment.Center,
                ) {
                    Text("No image selected", color = MaterialTheme.colorScheme.outlineVariant)
                }
            }

            Button(onClick = { imagePicker.launch("image/*") }, modifier = Modifier.fillMaxWidth()) {
                Text(if (selectedUri == null) "Choose Image" else "Change Image")
            }

            if (selectedUri != null) {
                if (uploading) {
                    CircularProgressIndicator()
                } else {
                    Button(
                        onClick = {
                            uploading = true
                            val bytes = context.contentResolver
                                .openInputStream(selectedUri!!)?.readBytes() ?: run {
                                uploading = false
                                return@Button
                            }
                            viewModel.uploadBanner(shopId, bytes)
                        },
                        modifier = Modifier.fillMaxWidth(),
                    ) {
                        Text("Upload Banner")
                    }
                }
            }

            Spacer(modifier = Modifier.height(8.dp))
            Text(
                "Recommended: 1200×400px, JPEG or PNG, max 5MB",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}
