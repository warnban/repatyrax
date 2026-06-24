package com.tyrax.presentation.components

import androidx.compose.animation.core.*
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.material3.Text
import com.tyrax.presentation.theme.TyraxColors
import com.tyrax.presentation.theme.TyraxTypography
import kotlinx.coroutines.delay

/**
 * Primary TYRAX action button. Sharp corners (0.dp radius). Aggressive.
 *
 * @param filled   true = Red background + White text. false = transparent + Red border + Red text.
 * @param loading  true = replaces label with an animated "..." ticker.
 */
@Composable
fun TyraxButton(
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    filled: Boolean = true,
    enabled: Boolean = true,
    loading: Boolean = false,
) {
    val containerAlpha = if (enabled) 1f else 0.4f

    Box(
        contentAlignment = Alignment.Center,
        modifier = modifier
            .alpha(containerAlpha)
            .then(
                if (filled) Modifier.background(TyraxColors.Red)
                else Modifier
                    .background(Color.Transparent)
                    .border(1.dp, TyraxColors.Red)
            )
            .clickable(enabled = enabled && !loading, onClick = onClick)
            .padding(horizontal = 32.dp, vertical = 16.dp),
    ) {
        if (loading) {
            LoadingDots()
        } else {
            Text(
                text      = label,
                style     = TyraxTypography.headline,
                color     = if (filled) TyraxColors.White else TyraxColors.Red,
                textAlign = TextAlign.Center,
            )
        }
    }
}

@Composable
private fun LoadingDots() {
    var dotCount by remember { mutableIntStateOf(1) }
    LaunchedEffect(Unit) {
        while (true) {
            delay(400)
            dotCount = (dotCount % 3) + 1
        }
    }
    Text(
        text      = ".".repeat(dotCount),
        style     = TyraxTypography.headline,
        color     = TyraxColors.White,
        textAlign = TextAlign.Center,
        modifier  = Modifier.defaultMinSize(minWidth = 32.dp),
    )
}

/**
 * Giant center ENTER / DISCONNECT button for the main screen.
 * Pulses red when active.
 */
@Composable
fun TyraxEnterButton(
    isConnected: Boolean,
    isConnecting: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val infiniteTransition = rememberInfiniteTransition(label = "pulse")
    val pulseAlpha by infiniteTransition.animateFloat(
        initialValue  = 0.6f,
        targetValue   = 1f,
        animationSpec = infiniteRepeatable(
            animation  = tween(800, easing = EaseInOut),
            repeatMode = RepeatMode.Reverse,
        ),
        label = "pulse_alpha",
    )

    val label = when {
        isConnecting -> "BREACHING…"
        isConnected  -> "DISCONNECT"
        else         -> "ENTER"
    }

    val borderColor = if (isConnected) TyraxColors.Red else TyraxColors.White
    val glowAlpha   = if (isConnected || isConnecting) pulseAlpha else 1f

    Box(
        contentAlignment = Alignment.Center,
        modifier = modifier
            .size(200.dp)
            .alpha(glowAlpha)
            .border(2.dp, borderColor)
            .background(Color.Transparent)
            .clickable(onClick = onClick),
    ) {
        Text(
            text      = label,
            style     = TyraxTypography.display,
            color     = if (isConnected) TyraxColors.Red else TyraxColors.White,
            textAlign = TextAlign.Center,
        )
    }
}
