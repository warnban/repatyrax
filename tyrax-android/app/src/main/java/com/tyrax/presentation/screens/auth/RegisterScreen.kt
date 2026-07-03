package com.tyrax.presentation.screens.auth

import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.tyrax.R
import com.tyrax.presentation.components.TyraxButton
import com.tyrax.presentation.components.TyraxTextField
import com.tyrax.presentation.theme.TyraxColors
import com.tyrax.presentation.theme.TyraxTypography

@Composable
fun RegisterScreen(
    onNavigateToMain: () -> Unit,
    onNavigateToLogin: () -> Unit,
    onNavigateToVerify: (String) -> Unit,
    viewModel: AuthViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()
    val context = LocalContext.current

    LaunchedEffect(Unit) {
        viewModel.events.collect { event ->
            when (event) {
                is AuthUiEvent.NavigateToMain -> onNavigateToMain()
                is AuthUiEvent.NavigateToVerify -> onNavigateToVerify(event.email)
                is AuthUiEvent.OpenUrl -> {
                    runCatching {
                        context.startActivity(
                            Intent(Intent.ACTION_VIEW, Uri.parse(event.url)).apply {
                                flags = Intent.FLAG_ACTIVITY_NEW_TASK
                            }
                        )
                    }
                }
            }
        }
    }

    var email by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var confirmPassword by remember { mutableStateOf("") }

    // Client-side password match validation; overrides server error display.
    var passwordMismatch by remember { mutableStateOf(false) }

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

            Spacer(modifier = Modifier.height(48.dp))

            TyraxTextField(
                value         = email,
                onValueChange = {
                    email = it
                    viewModel.clearError()
                    passwordMismatch = false
                },
                label           = stringResource(R.string.auth_label_login),
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Email),
                modifier        = Modifier.fillMaxWidth(),
            )

            Spacer(modifier = Modifier.height(24.dp))

            TyraxTextField(
                value                = password,
                onValueChange        = {
                    password = it
                    passwordMismatch = false
                    viewModel.clearError()
                },
                label                = stringResource(R.string.auth_label_password),
                visualTransformation = PasswordVisualTransformation(),
                keyboardOptions      = KeyboardOptions(keyboardType = KeyboardType.Password),
                modifier             = Modifier.fillMaxWidth(),
            )

            Spacer(modifier = Modifier.height(24.dp))

            TyraxTextField(
                value                = confirmPassword,
                onValueChange        = {
                    confirmPassword = it
                    passwordMismatch = false
                    viewModel.clearError()
                },
                label                = stringResource(R.string.auth_label_confirm_password),
                visualTransformation = PasswordVisualTransformation(),
                keyboardOptions      = KeyboardOptions(keyboardType = KeyboardType.Password),
                modifier             = Modifier.fillMaxWidth(),
            )

            // Inline validation feedback
            val errorText = when {
                passwordMismatch -> stringResource(R.string.error_passwords_mismatch)
                uiState is AuthUiState.Error -> (uiState as AuthUiState.Error).message
                else -> null
            }
            if (errorText != null) {
                Spacer(modifier = Modifier.height(16.dp))
                Text(
                    text      = errorText,
                    style     = TyraxTypography.label,
                    color     = TyraxColors.Red,
                    textAlign = TextAlign.Center,
                )
            }

            Spacer(modifier = Modifier.height(40.dp))

            TyraxButton(
                label   = stringResource(R.string.btn_register),
                onClick = {
                    if (password != confirmPassword) {
                        passwordMismatch = true
                    } else {
                        viewModel.register(email, password)
                    }
                },
                loading  = isLoading,
                modifier = Modifier.fillMaxWidth(),
            )

            Spacer(modifier = Modifier.height(24.dp))

            // Divider with "или"
            Row(
                verticalAlignment = Alignment.CenterVertically,
                modifier          = Modifier.fillMaxWidth(),
            ) {
                Box(
                    modifier = Modifier
                        .weight(1f)
                        .height(1.dp)
                        .background(TyraxColors.SubText),
                )
                Text(
                    text  = stringResource(R.string.label_or),
                    style = TyraxTypography.label,
                    color = TyraxColors.SubText,
                )
                Box(
                    modifier = Modifier
                        .weight(1f)
                        .height(1.dp)
                        .background(TyraxColors.SubText),
                )
            }

            Spacer(modifier = Modifier.height(24.dp))

            TyraxButton(
                label    = stringResource(R.string.auth_enter_via_telegram),
                onClick  = { viewModel.startTelegramAuth() },
                filled   = false,
                loading  = isLoading,
                modifier = Modifier.fillMaxWidth(),
            )

            Spacer(modifier = Modifier.height(16.dp))

            TelegramBotHint(
                onClick = {
                    runCatching {
                        context.startActivity(
                            Intent(Intent.ACTION_VIEW, Uri.parse("https://t.me/tyraxvpnbot")).apply {
                                flags = Intent.FLAG_ACTIVITY_NEW_TASK
                            }
                        )
                    }
                },
            )

            Spacer(modifier = Modifier.weight(1f))

            Text(
                text     = stringResource(R.string.auth_have_account),
                style    = TyraxTypography.label,
                color    = TyraxColors.SubText,
                modifier = Modifier
                    .clickable(onClick = onNavigateToLogin)
                    .padding(bottom = 40.dp, top = 8.dp),
            )
        }
    }
}
