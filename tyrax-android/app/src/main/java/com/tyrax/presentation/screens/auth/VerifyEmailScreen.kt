package com.tyrax.presentation.screens.auth

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.tyrax.R
import com.tyrax.presentation.components.TyraxButton
import com.tyrax.presentation.components.TyraxTextField
import com.tyrax.presentation.theme.TyraxColors
import com.tyrax.presentation.theme.TyraxTypography

/**
 * Email confirmation gate shown after registration (and after a login attempt by
 * an unconfirmed identity). The user enters the 6-digit code from the email; on
 * success the ViewModel opens a session and navigates to Main.
 */
@Composable
fun VerifyEmailScreen(
    email: String,
    onNavigateToMain: () -> Unit,
    onNavigateBack: () -> Unit,
    viewModel: AuthViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()

    LaunchedEffect(Unit) {
        viewModel.events.collect { event ->
            if (event is AuthUiEvent.NavigateToMain) onNavigateToMain()
        }
    }

    var code by remember { mutableStateOf("") }
    var resent by remember { mutableStateOf(false) }
    val isLoading = uiState is AuthUiState.Loading

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(TyraxColors.Black),
    ) {
        Column(
            horizontalAlignment = Alignment.CenterHorizontally,
            modifier = Modifier
                .fillMaxSize()
                .padding(horizontal = 32.dp),
        ) {
            Spacer(modifier = Modifier.height(72.dp))

            Text(
                text  = stringResource(R.string.label_brand),
                style = TyraxTypography.display,
            )

            Spacer(modifier = Modifier.height(40.dp))

            Text(
                text      = stringResource(R.string.verify_title),
                style     = TyraxTypography.headline,
                color     = TyraxColors.Red,
                textAlign = TextAlign.Center,
            )

            Spacer(modifier = Modifier.height(12.dp))

            Text(
                text      = stringResource(R.string.verify_subtitle, email),
                style     = TyraxTypography.label,
                color     = TyraxColors.SubText,
                textAlign = TextAlign.Center,
            )

            Spacer(modifier = Modifier.height(40.dp))

            TyraxTextField(
                value         = code,
                onValueChange = {
                    // Numeric-only, 6 digits max.
                    code = it.filter { ch -> ch.isDigit() }.take(6)
                    resent = false
                    viewModel.clearError()
                },
                label           = stringResource(R.string.verify_label_code),
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.NumberPassword),
                modifier        = Modifier.fillMaxWidth(),
            )

            if (uiState is AuthUiState.Error) {
                Spacer(modifier = Modifier.height(16.dp))
                Text(
                    text      = (uiState as AuthUiState.Error).message,
                    style     = TyraxTypography.label,
                    color     = TyraxColors.Red,
                    textAlign = TextAlign.Center,
                )
            }

            Spacer(modifier = Modifier.height(40.dp))

            TyraxButton(
                label    = stringResource(R.string.verify_btn_confirm),
                onClick  = { viewModel.verify(email, code) },
                loading  = isLoading,
                enabled  = code.length == 6,
                modifier = Modifier.fillMaxWidth(),
            )

            Spacer(modifier = Modifier.height(24.dp))

            Text(
                text      = stringResource(
                    if (resent) R.string.verify_resent else R.string.verify_resend
                ),
                style     = TyraxTypography.label,
                color     = if (resent) TyraxColors.SubText else TyraxColors.Red,
                textAlign = TextAlign.Center,
                modifier  = Modifier
                    .fillMaxWidth()
                    .clickable(enabled = !resent) {
                        viewModel.resend(email)
                        resent = true
                    }
                    .padding(8.dp),
            )

            Spacer(modifier = Modifier.weight(1f))

            Text(
                text     = stringResource(R.string.verify_back),
                style    = TyraxTypography.label,
                color    = TyraxColors.SubText,
                modifier = Modifier
                    .clickable(onClick = onNavigateBack)
                    .padding(bottom = 40.dp, top = 8.dp),
            )
        }
    }
}
