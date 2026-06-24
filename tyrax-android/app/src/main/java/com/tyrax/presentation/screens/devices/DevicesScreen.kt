package com.tyrax.presentation.screens.devices

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material3.AlertDialog
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
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.tyrax.R
import com.tyrax.domain.model.InviteRecord
import com.tyrax.domain.model.UserDevice
import com.tyrax.presentation.components.TyraxButton
import com.tyrax.presentation.components.TyraxDialog
import com.tyrax.presentation.theme.TyraxColors
import com.tyrax.presentation.theme.TyraxTypography
import kotlinx.coroutines.delay

@Composable
fun DevicesScreen(
    onNavigateBack: () -> Unit,
    viewModel: DevicesViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()

    var deviceToDelete by remember { mutableStateOf<UserDevice?>(null) }
    var inviteToRemove by remember { mutableStateOf<String?>(null) }
    var showInviteDialog by remember { mutableStateOf(false) }
    var inviteInput by remember { mutableStateOf("") }

    // Auto-dismiss "DEVICE ADDED" banner after 2s.
    LaunchedEffect(uiState.addedBanner) {
        if (uiState.addedBanner) {
            delay(2_000)
            viewModel.dismissBanner()
        }
    }

    // Delete confirmation dialog.
    deviceToDelete?.let { device ->
        TyraxDialog(
            title     = stringResource(R.string.dialog_remove_device_title),
            body      = device.name,
            confirmText = stringResource(R.string.btn_confirm),
            cancelText  = stringResource(R.string.btn_cancel),
            onConfirm = {
                viewModel.deleteDevice(device.id)
                deviceToDelete = null
            },
            onDismiss = { deviceToDelete = null },
        )
    }

    // Invite removal dialog.
    inviteToRemove?.let { accountId ->
        TyraxDialog(
            title     = stringResource(R.string.dialog_remove_invite_title),
            body      = accountId,
            confirmText = stringResource(R.string.btn_confirm),
            cancelText  = stringResource(R.string.btn_cancel),
            onConfirm = {
                viewModel.removeInvite(accountId)
                inviteToRemove = null
            },
            onDismiss = { inviteToRemove = null },
        )
    }

    // Send invite dialog.
    if (showInviteDialog) {
        AlertDialog(
            onDismissRequest = { showInviteDialog = false; inviteInput = "" },
            containerColor   = TyraxColors.DarkGray,
            title = {
                Text(text = stringResource(R.string.dialog_invite_title), style = TyraxTypography.headline, color = TyraxColors.White)
            },
            text = {
                Column {
                    Text(text = stringResource(R.string.label_account_id), style = TyraxTypography.label)
                    Spacer(modifier = Modifier.height(8.dp))
                    BasicTextField(
                        value         = inviteInput,
                        onValueChange = { inviteInput = it },
                        textStyle     = TyraxTypography.body.copy(color = TyraxColors.White),
                        cursorBrush   = SolidColor(TyraxColors.Red),
                        modifier      = Modifier
                            .fillMaxWidth()
                            .background(TyraxColors.MidGray)
                            .padding(8.dp),
                    )
                    uiState.inviteError?.let { err ->
                        Spacer(modifier = Modifier.height(8.dp))
                        Text(text = err, style = TyraxTypography.label, color = TyraxColors.Red)
                    }
                }
            },
            confirmButton = {
                Text(
                    text     = stringResource(R.string.btn_send),
                    style    = TyraxTypography.accent,
                    modifier = Modifier.clickable {
                        viewModel.sendInvite(inviteInput.trim())
                        showInviteDialog = false
                        inviteInput = ""
                    }.padding(8.dp),
                )
            },
            dismissButton = {
                Text(
                    text     = stringResource(R.string.btn_cancel),
                    style    = TyraxTypography.label,
                    modifier = Modifier.clickable { showInviteDialog = false }.padding(8.dp),
                )
            },
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
            verticalAlignment    = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween,
            modifier             = Modifier.fillMaxWidth(),
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

        // ── Added banner ───────────────────────────────────────────────────────
        if (uiState.addedBanner) {
            Spacer(modifier = Modifier.height(12.dp))
            Text(
                text      = stringResource(R.string.status_device_added),
                style     = TyraxTypography.label,
                color     = TyraxColors.Red,
                textAlign = TextAlign.Center,
                modifier  = Modifier.fillMaxWidth(),
            )
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

            // DOMINION invite section
            val isDominion = uiState.subscription?.tier?.uppercase() == "DOMINION"
            if (isDominion) {
                item {
                    Spacer(modifier = Modifier.height(24.dp))
                    Text(text = stringResource(R.string.label_invited_accounts), style = TyraxTypography.label)
                    Spacer(modifier = Modifier.height(8.dp))
                }
                items(uiState.invites, key = { "invite_${it.id}" }) { invite ->
                    InviteRow(invite = invite, onRemove = { inviteToRemove = invite.inviteeId })
                    HorizontalDivider(thickness = 0.5.dp, color = TyraxColors.MidGray)
                }
                item {
                    Spacer(modifier = Modifier.height(16.dp))
                    TyraxButton(
                        label    = stringResource(R.string.btn_invite_account),
                        onClick  = { showInviteDialog = true },
                        filled   = false,
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
            }
        }

        Spacer(modifier = Modifier.height(16.dp))

        TyraxButton(
            label    = stringResource(R.string.btn_add_device),
            onClick  = { viewModel.addDevice() },
            filled   = false,
            modifier = Modifier.fillMaxWidth(),
        )

        Spacer(modifier = Modifier.height(40.dp))
    }
}

@Composable
private fun DeviceRow(device: UserDevice, onDelete: () -> Unit) {
    Row(
        verticalAlignment    = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.SpaceBetween,
        modifier             = Modifier
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

@Composable
private fun InviteRow(invite: InviteRecord, onRemove: () -> Unit) {
    Row(
        verticalAlignment    = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.SpaceBetween,
        modifier             = Modifier
            .fillMaxWidth()
            .padding(vertical = 14.dp),
    ) {
        Column {
            Text(text = invite.inviteeId, style = TyraxTypography.body, color = TyraxColors.White)
            Text(text = invite.status.uppercase(), style = TyraxTypography.label)
        }
        Text(
            text     = stringResource(R.string.glyph_remove),
            style    = TyraxTypography.headline,
            color    = TyraxColors.Red,
            modifier = Modifier.clickable { onRemove() }.padding(8.dp),
        )
    }
}

