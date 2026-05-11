package com.mahaswarna.components

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.SignalWifiOff
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

/**
 * StaleRateBanner — shown when rate data may be outdated.
 *
 * Visibility rules (ANY of these → show):
 *   1. rate.isStale == true (backend field)
 *   2. wsState in {Reconnecting, Disconnected} for > 30 seconds
 *   3. wsState == Error (immediate, no 30s grace — terminal state)
 *   4. killSwitchWs == true (polling mode — permanently stale by definition)
 *   5. homeResponse._degraded == true (BFF partial upstream failure)
 *
 * isKillSwitchMode: when true, displays "Polling mode — live updates paused".
 * Default message: "Rates may be outdated — tap to refresh".
 *
 * This composable is stateless — caller drives visibility via conditional:
 *   if (staleBanner.showBanner) { StaleRateBanner(...) }
 */
@Composable
fun StaleRateBanner(
    modifier: Modifier = Modifier,
    isKillSwitchMode: Boolean = false,
    onTap: (() -> Unit)? = null,
) {
    val message = if (isKillSwitchMode)
        "Polling mode — live rate updates paused"
    else
        "Rates may be outdated"

    Surface(
        modifier = modifier,
        color = MaterialTheme.colorScheme.errorContainer,
        onClick = { onTap?.invoke() },
        enabled = onTap != null,
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Icon(
                imageVector = Icons.Default.SignalWifiOff,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onErrorContainer,
                modifier = Modifier.size(18.dp),
            )
            Text(
                text = message,
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onErrorContainer,
            )
        }
    }
}
