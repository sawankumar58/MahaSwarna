package com.mahaswarna.components

import androidx.compose.foundation.layout.*
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ErrorOutline
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

/**
 * ErrorRetryCard — shown when a network request fails and the caller wants a
 * consistent full-screen (or card-sized) error + retry affordance.
 *
 * Usage:
 *   ErrorRetryCard(message = "Could not load rates", onRetry = viewModel::retry)
 *
 * This composable is stateless — caller drives visibility and the retry action.
 */
@Composable
fun ErrorRetryCard(
    message: String,
    onRetry: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier.padding(32.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Icon(
            imageVector = Icons.Default.ErrorOutline,
            contentDescription = null,
            tint = MaterialTheme.colorScheme.error,
            modifier = Modifier.size(48.dp),
        )
        Text(
            text = message,
            style = MaterialTheme.typography.bodyLarge,
            color = MaterialTheme.colorScheme.onSurface,
        )
        Button(onClick = onRetry) {
            Text("Retry")
        }
    }
}
