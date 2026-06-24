package com.tyrax.presentation.components

import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.text.TextStyle
import com.tyrax.presentation.theme.TyraxColors
import com.tyrax.presentation.theme.TyraxTypography
import kotlinx.coroutines.delay

/**
 * Text that plays a rapid 3-frame alpha glitch on first composition,
 * then settles to fully opaque. Glitch total duration ≈ 150ms.
 */
@Composable
fun GlitchText(
    text: String,
    modifier: Modifier = Modifier,
    style: TextStyle = TyraxTypography.display,
) {
    var targetAlpha by remember { mutableFloatStateOf(0f) }
    val alpha by animateFloatAsState(
        targetValue   = targetAlpha,
        animationSpec = tween(durationMillis = 50),
        label         = "glitch_alpha",
    )

    LaunchedEffect(text) {
        repeat(3) {
            targetAlpha = 0f
            delay(50)
            targetAlpha = 1f
            delay(50)
        }
        targetAlpha = 1f
    }

    Text(
        text     = text,
        style    = style,
        modifier = modifier.alpha(alpha),
    )
}
