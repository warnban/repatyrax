package com.tyrax.presentation.screens.settings

import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.tyrax.R
import com.tyrax.presentation.components.TyraxDialog
import com.tyrax.presentation.theme.TyraxColors
import com.tyrax.presentation.theme.TyraxTypography

@Composable
fun SettingsScreen(
    onNavigateBack: () -> Unit,
    onNavigateToSubscription: () -> Unit,
    onNavigateToDevices: () -> Unit,
    onLoggedOut: () -> Unit,
    viewModel: SettingsViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()
    val context = LocalContext.current
    var showLogoutDialog by remember { mutableStateOf(false) }

    LaunchedEffect(Unit) {
        viewModel.events.collect { event ->
            when (event) {
                is SettingsUiEvent.OpenUrl -> {
                    context.startActivity(
                        Intent(Intent.ACTION_VIEW, Uri.parse(event.url)).apply {
                            flags = Intent.FLAG_ACTIVITY_NEW_TASK
                        }
                    )
                }
            }
        }
    }

    if (uiState.loggedOut) {
        onLoggedOut()
    }

    if (showLogoutDialog) {
        TyraxDialog(
            title       = stringResource(R.string.dialog_exit_title),
            body        = stringResource(R.string.dialog_exit_body),
            confirmText = stringResource(R.string.btn_confirm),
            cancelText  = stringResource(R.string.btn_cancel),
            onConfirm   = { viewModel.logout(); showLogoutDialog = false },
            onDismiss   = { showLogoutDialog = false },
        )
    }

    val emptyValue = stringResource(R.string.value_empty)

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(TyraxColors.Black)
            .padding(horizontal = 24.dp),
    ) {
        Spacer(modifier = Modifier.height(52.dp))

        Row(
            verticalAlignment = Alignment.CenterVertically,
            modifier = Modifier.fillMaxWidth(),
        ) {
            Text(
                text     = stringResource(R.string.glyph_back),
                style    = TyraxTypography.headline,
                modifier = Modifier.clickable { onNavigateBack() }.padding(end = 16.dp),
            )
            Text(text = stringResource(R.string.nav_settings), style = TyraxTypography.headline)
        }

        Spacer(modifier = Modifier.height(24.dp))

        // ── АККАУНТ ───────────────────────────────────────────────────────────
        SectionHeader(stringResource(R.string.settings_section_account))
        SettingsRow(label = stringResource(R.string.settings_label_email), value = uiState.email ?: emptyValue, onClick = null)
        HorizontalDivider(thickness = 0.5.dp, color = TyraxColors.MidGray)

        if (uiState.telegramLinked) {
            SettingsRow(
                label   = stringResource(R.string.settings_telegram_linked),
                value   = "",
                color   = TyraxColors.White,
                onClick = null,
            )
        } else {
            SettingsRow(
                label   = stringResource(R.string.settings_link_telegram),
                value   = "",
                onClick = { viewModel.linkTelegram() },
            )
        }
        HorizontalDivider(thickness = 0.5.dp, color = TyraxColors.MidGray)

        Spacer(modifier = Modifier.height(24.dp))

        // ── ПОДПИСКА ──────────────────────────────────────────────────────────
        SectionHeader(stringResource(R.string.settings_section_subscription))
        SettingsRow(
            label   = stringResource(R.string.settings_label_tier),
            value   = uiState.tier ?: emptyValue,
            onClick = onNavigateToSubscription,
        )
        HorizontalDivider(thickness = 0.5.dp, color = TyraxColors.MidGray)

        Spacer(modifier = Modifier.height(24.dp))

        // ── УСТРОЙСТВА ────────────────────────────────────────────────────────
        SectionHeader(stringResource(R.string.settings_section_devices))
        SettingsRow(
            label   = stringResource(R.string.settings_label_devices),
            value   = uiState.devicesInfo ?: emptyValue,
            onClick = onNavigateToDevices,
        )
        HorizontalDivider(thickness = 0.5.dp, color = TyraxColors.MidGray)

        Spacer(modifier = Modifier.height(32.dp))

        SettingsRow(
            label   = stringResource(R.string.btn_exit_system),
            value   = "",
            color   = TyraxColors.Red,
            onClick = { showLogoutDialog = true },
        )
        HorizontalDivider(thickness = 0.5.dp, color = TyraxColors.MidGray)
    }
}

@Composable
private fun SectionHeader(text: String) {
    Text(
        text     = text,
        style    = TyraxTypography.label,
        color    = TyraxColors.SubText,
        modifier = Modifier.padding(bottom = 4.dp),
    )
}

@Composable
private fun SettingsRow(
    label: String,
    value: String,
    color: Color = TyraxColors.White,
    onClick: (() -> Unit)?,
) {
    Row(
        verticalAlignment = Alignment.CenterVertically,
        modifier = Modifier
            .fillMaxWidth()
            .then(if (onClick != null) Modifier.clickable { onClick() } else Modifier)
            .padding(vertical = 18.dp),
    ) {
        Text(
            text     = label,
            style    = TyraxTypography.label,
            color    = color,
            modifier = Modifier.weight(1f),
        )
        if (value.isNotBlank()) {
            Text(
                text  = value,
                style = TyraxTypography.label,
                color = TyraxColors.SubText,
            )
        }
    }
}
