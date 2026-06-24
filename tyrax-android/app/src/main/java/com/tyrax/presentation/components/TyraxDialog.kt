package com.tyrax.presentation.components

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.tyrax.presentation.theme.TyraxColors
import com.tyrax.presentation.theme.TyraxTypography

/**
 * TYRAX-branded confirmation dialog.
 * Dark background, sharp, no icons — cold and direct.
 */
@Composable
fun TyraxDialog(
    title: String,
    body: String,
    confirmText: String,
    cancelText: String,
    onConfirm: () -> Unit,
    onDismiss: () -> Unit,
) {
    AlertDialog(
        onDismissRequest = onDismiss,
        containerColor   = TyraxColors.DarkGray,
        title = {
            Text(text = title, style = TyraxTypography.headline, color = TyraxColors.White)
        },
        text = {
            Text(text = body, style = TyraxTypography.body, color = TyraxColors.SubText)
        },
        confirmButton = {
            Text(
                text     = confirmText,
                style    = TyraxTypography.accent,
                modifier = Modifier.clickable { onConfirm() }.padding(8.dp),
            )
        },
        dismissButton = {
            Text(
                text     = cancelText,
                style    = TyraxTypography.label,
                modifier = Modifier.clickable { onDismiss() }.padding(8.dp),
            )
        },
    )
}
