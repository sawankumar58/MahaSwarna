package com.mahaswarna.feature.auth.ui

import android.content.Intent
import android.net.Uri
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.layout.*
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.SystemUpdate
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp

/**
 * Non-dismissible update screen. Shown when VersionInterceptor or ApiErrorBus emits
 * ApiError.VersionDeprecated (HTTP 410 from any API call).
 *
 * Navigation rule (AppNavGraph):
 *   navController.navigate(Route.UpdateRequired) {
 *       popUpTo(Route.Home) { inclusive = true }
 *   }
 * This clears the entire back stack so the user cannot back-navigate out.
 *
 * BackHandler is also intercepted here as a belt-and-suspenders measure.
 * The Play Store link opens in the browser; there is no in-app update flow.
 */
@Composable
fun UpdateRequiredScreen() {
    // Block all back navigation — user MUST update
    BackHandler(enabled = true) { /* blocked */ }

    val context = LocalContext.current

    Scaffold { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .padding(horizontal = 32.dp),
            verticalArrangement = Arrangement.Center,
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Icon(
                imageVector = Icons.Default.SystemUpdate,
                contentDescription = null,
                modifier = Modifier.size(72.dp),
                tint = MaterialTheme.colorScheme.primary,
            )
            Spacer(Modifier.height(24.dp))
            Text(
                text = "Update Required",
                style = MaterialTheme.typography.headlineMedium,
                textAlign = TextAlign.Center,
            )
            Spacer(Modifier.height(16.dp))
            Text(
                text = "This version of MahaSwarna is no longer supported. " +
                       "Please update to the latest version to continue.",
                style = MaterialTheme.typography.bodyMedium,
                textAlign = TextAlign.Center,
            )
            Spacer(Modifier.height(32.dp))
            Button(
                onClick = {
                    val intent = Intent(
                        Intent.ACTION_VIEW,
                        Uri.parse("https://play.google.com/store/apps/details?id=com.mahaswarna")
                    )
                    context.startActivity(intent)
                },
                modifier = Modifier.fillMaxWidth(),
            ) {
                Text("Update on Play Store")
            }
        }
    }
}
