package com.tyrax.presentation.components

import androidx.compose.animation.core.withInfiniteAnimationFrameMillis
import androidx.compose.foundation.Canvas
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.nativeCanvas
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.unit.dp
import com.tyrax.presentation.theme.TyraxColors

/**
 * Subtle "digital rain": faint red glyphs falling top→bottom, drawn behind the
 * main content while the protocol is active. Pure decoration — no state, no I/O.
 * Kept low-alpha so it reads as ambient texture, never competing with the UI.
 */
@Composable
fun MatrixRainBackground(
    modifier: Modifier = Modifier,
    color: Color = TyraxColors.Red,
) {
    val glyphs = remember {
        "01<>/\\[]{}#*+=$%&:;ﾊﾐﾋｰｳｼﾅﾉﾎｱｶﾀﾃ".toCharArray()
    }
    val density = LocalDensity.current
    val cellPx = with(density) { 18.dp.toPx() }
    val textPx = with(density) { 15.dp.toPx() }

    val paint = remember {
        android.graphics.Paint().apply {
            textSize = textPx
            typeface = android.graphics.Typeface.MONOSPACE
            isAntiAlias = true
        }
    }

    // Monotonic clock in seconds; drives the fall without holding UI state.
    val time by produceState(0f) {
        val start = withInfiniteAnimationFrameMillis { it }
        while (true) {
            withInfiniteAnimationFrameMillis { value = (it - start) / 1000f }
        }
    }

    Canvas(modifier = modifier) {
        val cols = (size.width / cellPx).toInt().coerceAtLeast(1)
        val rows = (size.height / cellPx).toInt().coerceAtLeast(1)
        val tail = 14
        val cycle = rows + tail
        val baseArgb = color

        for (c in 0 until cols) {
            // Deterministic per-column speed & phase → stable, varied streams.
            val speed = 5f + (c * 37 % 11)        // rows per second
            val phase = (c * 53 % cycle)
            val headRaw = (time * speed + phase).toInt()
            val headRow = ((headRaw % cycle) + cycle) % cycle

            for (i in 0 until tail) {
                val row = headRow - i
                if (row < 0 || row > rows) continue

                val alpha = if (i == 0) 0.32f else (1f - i / tail.toFloat()) * 0.16f
                paint.color = baseArgb.copy(alpha = alpha).toArgb()

                val glyphIndex = (((c * 31 + row) % glyphs.size) + glyphs.size) % glyphs.size
                drawContext.canvas.nativeCanvas.drawText(
                    glyphs[glyphIndex].toString(),
                    c * cellPx,
                    row * cellPx + textPx,
                    paint,
                )
            }
        }
    }
}
