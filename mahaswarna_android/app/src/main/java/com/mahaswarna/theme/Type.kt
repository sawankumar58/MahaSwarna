package com.mahaswarna.theme

import androidx.compose.material3.Typography
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp
import com.mahaswarna.R

// Noto Serif for headings (brand weight), Roboto for body (legibility on budget screens)
val NotoSerif = FontFamily(
    Font(R.font.noto_serif_regular, FontWeight.Normal),
    Font(R.font.noto_serif_bold,    FontWeight.Bold),
)

val MahaSwarnTypography = Typography(
    displayLarge  = TextStyle(fontFamily = NotoSerif, fontWeight = FontWeight.Bold,  fontSize = 57.sp),
    headlineLarge = TextStyle(fontFamily = NotoSerif, fontWeight = FontWeight.Bold,  fontSize = 32.sp),
    headlineMedium= TextStyle(fontFamily = NotoSerif, fontWeight = FontWeight.SemiBold, fontSize = 28.sp),
    titleLarge    = TextStyle(fontFamily = NotoSerif, fontWeight = FontWeight.SemiBold, fontSize = 22.sp),
    bodyLarge     = TextStyle(fontFamily = FontFamily.Default, fontWeight = FontWeight.Normal, fontSize = 16.sp),
    bodyMedium    = TextStyle(fontFamily = FontFamily.Default, fontWeight = FontWeight.Normal, fontSize = 14.sp),
    labelLarge    = TextStyle(fontFamily = FontFamily.Default, fontWeight = FontWeight.Medium, fontSize = 14.sp),
)
