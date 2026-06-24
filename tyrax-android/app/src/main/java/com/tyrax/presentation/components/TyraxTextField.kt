package com.tyrax.presentation.components

import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import com.tyrax.presentation.theme.TyraxColors
import com.tyrax.presentation.theme.TyraxTypography

/**
 * TYRAX-branded text input.
 * Displays only a bottom border (no box/outline). Border turns Red on focus.
 * No rounded corners anywhere.
 */
@Composable
fun TyraxTextField(
    value: String,
    onValueChange: (String) -> Unit,
    label: String,
    modifier: Modifier = Modifier,
    visualTransformation: VisualTransformation = VisualTransformation.None,
    keyboardOptions: KeyboardOptions = KeyboardOptions.Default,
    keyboardActions: KeyboardActions = KeyboardActions.Default,
    singleLine: Boolean = true,
) {
    val interactionSource = remember { MutableInteractionSource() }
    val isFocused by interactionSource.collectIsFocusedAsState()
    val lineColor = if (isFocused) TyraxColors.Red else TyraxColors.SubText

    Column(modifier = modifier) {
        Text(
            text  = label,
            style = TyraxTypography.label,
            color = if (isFocused) TyraxColors.Red else TyraxColors.SubText,
        )

        Spacer(modifier = Modifier.height(6.dp))

        BasicTextField(
            value                = value,
            onValueChange        = onValueChange,
            textStyle            = TyraxTypography.body.copy(color = TyraxColors.White),
            cursorBrush          = SolidColor(TyraxColors.Red),
            visualTransformation = visualTransformation,
            keyboardOptions      = keyboardOptions,
            keyboardActions      = keyboardActions,
            singleLine           = singleLine,
            interactionSource    = interactionSource,
            modifier = Modifier
                .fillMaxWidth()
                .drawBehind {
                    drawLine(
                        color       = lineColor,
                        start       = Offset(0f, size.height),
                        end         = Offset(size.width, size.height),
                        strokeWidth = 1.dp.toPx(),
                    )
                }
                .padding(bottom = 8.dp),
        )
    }
}
