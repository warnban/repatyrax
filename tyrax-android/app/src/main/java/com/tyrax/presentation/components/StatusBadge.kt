package com.tyrax.presentation.components

import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.tyrax.R
import com.tyrax.presentation.theme.TyraxColors
import com.tyrax.presentation.theme.TyraxTypography

/**
 * Node status badge per TYRAX vocabulary.
 *
 * OPEN              → white border, white text   (system accessible)
 * MONITORED         → dim border + dim text      (restricted access)
 * HEAVILY_RESTRICTED→ red border, red text       (danger zone)
 */
@Composable
fun StatusBadge(
    status: String,
    modifier: Modifier = Modifier,
) {
    val (borderColor, textColor) = when (status.uppercase()) {
        "OPEN"               -> TyraxColors.White   to TyraxColors.White
        "MONITORED"          -> TyraxColors.SubText to TyraxColors.SubText
        "HEAVILY_RESTRICTED" -> TyraxColors.Red     to TyraxColors.Red
        else                 -> TyraxColors.SubText to TyraxColors.SubText
    }
    val label = when (status.uppercase()) {
        "OPEN"               -> stringResource(R.string.node_open)
        "MONITORED"          -> stringResource(R.string.node_monitored)
        "HEAVILY_RESTRICTED" -> stringResource(R.string.node_heavily_restricted)
        else                 -> status.uppercase()
    }

    Box(
        modifier = modifier
            .border(1.dp, borderColor)
            .padding(horizontal = 8.dp, vertical = 4.dp),
    ) {
        Text(
            text  = label,
            style = TyraxTypography.label,
            color = textColor,
        )
    }
}
