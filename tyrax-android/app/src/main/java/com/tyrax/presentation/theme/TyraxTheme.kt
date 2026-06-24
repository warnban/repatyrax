package com.tyrax.presentation.theme

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp

object TyraxColors {
    val Black    = Color(0xFF000000)
    val White    = Color(0xFFFFFFFF)
    val Red      = Color(0xFFFF1E1E)
    val DarkGray = Color(0xFF111111)
    val MidGray  = Color(0xFF1A1A1A)
    val SubText  = Color(0xFF555555)
    // In TYRAX, red IS success — the system is penetrated.
    val Success  = Color(0xFFFF1E1E)
}

private val TyraxColorScheme = darkColorScheme(
    background      = TyraxColors.Black,
    surface         = TyraxColors.DarkGray,
    surfaceVariant  = TyraxColors.MidGray,
    primary         = TyraxColors.Red,
    onPrimary       = TyraxColors.White,
    onBackground    = TyraxColors.White,
    onSurface       = TyraxColors.White,
    error           = TyraxColors.Red,
    onError         = TyraxColors.White,
)

object TyraxTypography {
    val display = TextStyle(
        fontWeight    = FontWeight.Black,
        fontSize      = 48.sp,
        letterSpacing = 4.sp,
        color         = TyraxColors.White,
    )
    val headline = TextStyle(
        fontWeight    = FontWeight.Black,
        fontSize      = 20.sp,
        letterSpacing = 3.sp,
        color         = TyraxColors.White,
    )
    val label = TextStyle(
        fontWeight    = FontWeight.Bold,
        fontSize      = 11.sp,
        letterSpacing = 2.5.sp,
        color         = TyraxColors.SubText,
    )
    val body = TextStyle(
        fontWeight    = FontWeight.Normal,
        fontSize      = 14.sp,
        letterSpacing = 0.5.sp,
        color         = TyraxColors.White,
    )
    val accent = TextStyle(
        fontWeight    = FontWeight.Black,
        fontSize      = 13.sp,
        letterSpacing = 2.sp,
        color         = TyraxColors.Red,
    )
}

@Composable
fun TyraxTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = TyraxColorScheme,
        content     = content,
    )
}
