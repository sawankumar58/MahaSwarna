package com.mahaswarna.theme

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

// Color palette vals are defined in Color.kt (same package — no import needed).

private val DarkColorScheme = darkColorScheme(
    primary          = MsGold,
    onPrimary        = MsOnGold,
    primaryContainer = MsGoldContainer,
    secondary        = MsSilver,
    secondaryContainer = MsSilverContainer,
    background       = MsBackground,
    surface          = MsSurface,
    onBackground     = MsOnBackground,
    onSurface        = MsOnBackground,
    error            = MsError,
)

private val LightColorScheme = lightColorScheme(
    primary          = Color(0xFF8B6000),
    onPrimary        = Color(0xFFFFFFFF),
    primaryContainer = Color(0xFFFFDFA0),
    secondary        = Color(0xFF5C6770),
    secondaryContainer = Color(0xFFDDE3EB),
    background       = Color(0xFFFFFBF0),
    surface          = Color(0xFFFFFBF0),
    onBackground     = Color(0xFF1C1B17),
    onSurface        = Color(0xFF1C1B17),
    error            = Color(0xFFBA1A1A),
)

@Composable
fun MahaSwarnTheme(
    darkTheme: Boolean = androidx.compose.foundation.isSystemInDarkTheme(),
    content: @Composable () -> Unit,
) {
    val colorScheme = if (darkTheme) DarkColorScheme else LightColorScheme
    MaterialTheme(
        colorScheme = colorScheme,
        typography  = MahaSwarnTypography,
        shapes      = MahaSwarnShapes,
        content     = content,
    )
}
