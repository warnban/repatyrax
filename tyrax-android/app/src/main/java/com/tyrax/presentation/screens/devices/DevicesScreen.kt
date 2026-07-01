package com.tyrax.presentation.screens.devices

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.tyrax.R
import com.tyrax.domain.model.UserDevice
import com.tyrax.presentation.components.TyraxDialog
import com.tyrax.presentation.theme.TyraxColors
import com.tyrax.presentation.theme.TyraxTypography

/**
 * Read-only device roster. Devices self-register on login (see MainViewModel), so
 * this screen only lists the account's devices, shows the tier's slot usage and
 * lets the user free a slot by removing a device.
 */
@Composable
fun DevicesScreen(
    onNavigateBack: () -> Unit,
    viewModel: DevicesViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()

    var deviceToDelete by remember { mutableStateOf<UserDevice?>(null) }

    deviceToDelete?.let { device ->
        TyraxDialog(
            title       = stringResource(R.string.dialog_remove_device_title),
            body        = device.name,
            confirmText = stringResource(R.string.btn_confirm),
            cancelText  = stringResource(R.string.btn_cancel),
            onConfirm   = {
                viewModel.deleteDevice(device.id)
                deviceToDelete = null
            },
            onDismiss   = { deviceToDelete = null },
        )
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(TyraxColors.Black)
            .padding(horizontal = 24.dp),
    ) {
        Spacer(modifier = Modifier.height(52.dp))

        // ── Header ─────────────────────────────────────────────────────────────
        Row(
            verticalAlignment     = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween,
            modifier              = Modifier.fillMaxWidth(),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    text     = stringResource(R.string.glyph_back),
                    style    = TyraxTypography.headline,
                    modifier = Modifier.clickable { onNavigateBack() }.padding(end = 16.dp),
                )
                Text(text = stringResource(R.string.label_my_devices), style = TyraxTypography.headline)
            }
            uiState.subscription?.let { sub ->
                Text(
                    text  = stringResource(R.string.label_device_count, sub.devicesCount, sub.devicesLimit),
                    style = TyraxTypography.accent,
                )
            }
        }

        uiState.error?.let { err ->
            Spacer(modifier = Modifier.height(12.dp))
            Text(
                text      = err,
                style     = TyraxTypography.label,
                color     = TyraxColors.Red,
                textAlign = TextAlign.Center,
                modifier  = Modifier.fillMaxWidth(),
            )
        }

        Spacer(modifier = Modifier.height(24.dp))

        // ── Device list ────────────────────────────────────────────────────────
        LazyColumn(modifier = Modifier.weight(1f)) {
            items(uiState.devices, key = { it.id }) { device ->
                DeviceRow(device = device, onDelete = { deviceToDelete = device })
                HorizontalDivider(thickness = 0.5.dp, color = TyraxColors.MidGray)
            }
        }

        Spacer(modifier = Modifier.height(40.dp))
    }
}

@Composable
private fun DeviceRow(device: UserDevice, onDelete: () -> Unit) {
    Row(
        verticalAlignment     = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.SpaceBetween,
        modifier              = Modifier
            .fillMaxWidth()
            .padding(vertical = 14.dp),
    ) {
        Column {
            Text(text = device.name, style = TyraxTypography.body, color = TyraxColors.White)
            if (device.createdAt.isNotBlank()) {
                Text(text = device.createdAt, style = TyraxTypography.label)
            }
        }
        Text(
            text     = stringResource(R.string.glyph_remove),
            style    = TyraxTypography.headline,
            color    = TyraxColors.Red,
            modifier = Modifier.clickable { onDelete() }.padding(8.dp),
        )
    }
}
