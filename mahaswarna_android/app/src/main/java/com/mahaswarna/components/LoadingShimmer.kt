package com.mahaswarna.components

import androidx.compose.animation.core.*
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.unit.dp

/**
 * LoadingShimmer — shown on first install when Room is empty.
 *
 * HomeViewModel enforces a 2-second hard timeout — if BFF hasn't responded in 2s,
 * state transitions to NoDataAvailable. This component never persists longer than 2s
 * thanks to that timeout; it does NOT enforce the timeout itself.
 */
@Composable
fun LoadingShimmer(modifier: Modifier = Modifier) {
    Column(
        modifier = modifier.padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        // Simulated rate card skeleton
        ShimmerCard(height = 120)
        ShimmerCard(height = 120)
        ShimmerCard(height = 80)
    }
}

@Composable
private fun ShimmerCard(height: Int) {
    Box(
        modifier = Modifier
            .fillMaxWidth()
            .height(height.dp)
            .shimmerEffect()
            .background(
                color = MaterialTheme.colorScheme.surfaceVariant,
                shape = RoundedCornerShape(12.dp),
            ),
    )
}

private fun Modifier.shimmerEffect(): Modifier = composed {
    val transition = rememberInfiniteTransition(label = "shimmer")
    val translateAnim by transition.animateFloat(
        initialValue = 0f,
        targetValue  = 1000f,
        animationSpec = infiniteRepeatable(
            animation = tween(durationMillis = 1200, easing = FastOutSlowInEasing),
            repeatMode = RepeatMode.Restart,
        ),
        label = "shimmer_translate",
    )
    background(
        brush = Brush.linearGradient(
            colors = listOf(
                MaterialTheme.colorScheme.surfaceVariant,
                MaterialTheme.colorScheme.surface,
                MaterialTheme.colorScheme.surfaceVariant,
            ),
            start = Offset(translateAnim - 300f, 0f),
            end   = Offset(translateAnim, 0f),
        ),
        shape = RoundedCornerShape(12.dp),
    )
}
