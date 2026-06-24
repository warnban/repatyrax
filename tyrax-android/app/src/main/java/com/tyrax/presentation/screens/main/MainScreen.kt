package com.tyrax.presentation.screens.main

import androidx.compose.animation.core.EaseInOut
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.core.animateFloat
import android.app.Activity
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.runtime.key
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.tyrax.R
import com.tyrax.domain.model.VpnState
import com.tyrax.presentation.components.GlitchText
import com.tyrax.presentation.theme.TyraxColors
import com.tyrax.presentation.theme.TyraxTypography
import kotlinx.coroutines.delay

@Composable
fun MainScreen(
    onNavigateToNodes: () -> Unit,
    onNavigateToSubscription: () -> Unit,
    onNavigateToSettings: () -> Unit = {},
    viewModel: MainViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()

    // System VPN consent dialog. On approval, resume the pending connection.
    val vpnPermissionLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.StartActivityForResult(),
    ) { result ->
        if (result.resultCode == Activity.RESULT_OK) {
            viewModel.onPermissionGranted()
        }
    }

    // Launch the consent dialog whenever the tunnel reports it needs permission.
    LaunchedEffect(uiState.vpnState) {
        (uiState.vpnState as? VpnState.NeedsPermission)?.let { needs ->
            vpnPermissionLauncher.launch(needs.intent)
        }
    }

    // Local display text for the connection animation sequence.
    // null = derive from vpnState; non-null = transient override (NODE ACQUIRED).
    // Stored as a string resource id so it can be set outside composable scope.
    var statusOverrideRes by remember { mutableStateOf<Int?>(null) }
    // Incrementing this key re-triggers the GlitchText animation.
    var glitchKey by remember { mutableIntStateOf(0) }

    // Animation sequence triggered on vpnState transitions.
    LaunchedEffect(uiState.vpnState) {
        when (uiState.vpnState) {
            is VpnState.Connecting -> {
                glitchKey++         // glitch on BREACHING NETWORK
                statusOverrideRes = null
            }
            is VpnState.Connected -> {
                statusOverrideRes = R.string.status_node_acquired
                glitchKey++
                delay(500)
                statusOverrideRes = null   // now shows ACCESS GRANTED
                glitchKey++
            }
            else -> {
                statusOverrideRes = null
            }
        }
    }

    val statusText = statusOverrideRes?.let { stringResource(it) } ?: when (uiState.vpnState) {
        is VpnState.Connected    -> stringResource(R.string.status_access_granted)
        is VpnState.Connecting   -> stringResource(R.string.status_breaching)
        is VpnState.Reconnecting -> stringResource(R.string.status_reconnecting)
        is VpnState.Error        -> stringResource(R.string.status_connection_failed)
        else                     -> stringResource(R.string.status_outside_system)
    }

    val statusColor = when (uiState.vpnState) {
        is VpnState.Connected    -> if (statusOverrideRes == null) TyraxColors.Red else TyraxColors.White
        is VpnState.Connecting,
        is VpnState.Reconnecting -> TyraxColors.White
        else                     -> TyraxColors.SubText
    }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(TyraxColors.Black),
    ) {
        Column(
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.SpaceBetween,
            modifier = Modifier
                .fillMaxSize()
                .padding(horizontal = 24.dp),
        ) {

            // ── TOP ZONE — status ──────────────────────────────────────────────
            Column(
                horizontalAlignment = Alignment.CenterHorizontally,
                modifier = Modifier.padding(top = 72.dp),
            ) {
                Text(
                    text  = stringResource(R.string.label_status),
                    style = TyraxTypography.label,
                )
                Spacer(modifier = Modifier.height(8.dp))
                // Re-keying reruns GlitchText composition and replays the flicker.
                key(glitchKey) {
                    GlitchText(
                        text  = statusText,
                        style = TyraxTypography.display.copy(color = statusColor),
                    )
                }
            }

            // ── CENTER ZONE — 200×200dp main button ───────────────────────────
            MainButton(
                vpnState = uiState.vpnState,
                onConnect    = { viewModel.connect() },
                onDisconnect = { viewModel.disconnect() },
            )

            // ── BOTTOM ZONE — node info + nav ──────────────────────────────────
            Column(
                horizontalAlignment = Alignment.CenterHorizontally,
                modifier = Modifier.padding(bottom = 52.dp),
            ) {
                // Node info doubles as the entry point to the node list.
                if (uiState.vpnState is VpnState.Connected) {
                    Text(
                        text     = stringResource(R.string.label_node_status, uiState.currentNode),
                        style    = TyraxTypography.label,
                        modifier = Modifier.clickable { onNavigateToNodes() },
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text  = stringResource(R.string.label_ping_ms, uiState.pingMs),
                        style = TyraxTypography.accent,
                    )
                } else {
                    Text(
                        text     = stringResource(R.string.label_node_none),
                        style    = TyraxTypography.label,
                        modifier = Modifier.clickable { onNavigateToNodes() },
                    )
                }

                // FREE-tier traffic counter
                if (uiState.tier.uppercase() == "FREE") {
                    Spacer(modifier = Modifier.height(16.dp))
                    TrafficCounter(
                        usedBytes  = uiState.trafficUsedBytes,
                        limitBytes = uiState.trafficLimitBytes,
                    )
                }

                Spacer(modifier = Modifier.height(32.dp))

                // Bottom nav: home marker (current screen, no label) · ТАРИФЫ · НАСТРОЙКИ
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(32.dp),
                ) {
                    Box(
                        modifier = Modifier
                            .size(8.dp)
                            .background(TyraxColors.Red),
                    )
                    Text(
                        text     = stringResource(R.string.nav_control),
                        style    = TyraxTypography.label,
                        modifier = Modifier.clickable { onNavigateToSubscription() },
                    )
                    Text(
                        text     = stringResource(R.string.nav_settings),
                        style    = TyraxTypography.label,
                        modifier = Modifier.clickable { onNavigateToSettings() },
                    )
                }
            }
        }
    }
}

// ── Traffic counter (FREE tier) ────────────────────────────────────────────────

@Composable
private fun TrafficCounter(usedBytes: Long, limitBytes: Long) {
    val usedGb  = usedBytes.toFloat()  / (1024f * 1024f * 1024f)
    val limitGb = limitBytes.toFloat() / (1024f * 1024f * 1024f)
    val fraction = if (limitBytes > 0) (usedBytes.toFloat() / limitBytes.toFloat()).coerceIn(0f, 1f) else 0f

    val trackWidth by animateFloatAsState(
        targetValue = fraction,
        animationSpec = tween(400),
        label = "traffic_bar",
    )

    Column(
        horizontalAlignment = Alignment.CenterHorizontally,
        modifier = Modifier.fillMaxWidth(),
    ) {
        Text(
            text  = stringResource(
                R.string.label_traffic,
                "%.1f".format(usedGb),
                "%.0f".format(limitGb),
            ),
            style = TyraxTypography.label,
        )
        Spacer(modifier = Modifier.height(6.dp))
        // Track
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .height(2.dp)
                .background(TyraxColors.MidGray),
        ) {
            // Fill
            Box(
                modifier = Modifier
                    .fillMaxWidth(fraction = trackWidth)
                    .height(2.dp)
                    .background(TyraxColors.Red),
            )
        }
    }
}

// ── Main button ────────────────────────────────────────────────────────────────

@Composable
private fun MainButton(
    vpnState: VpnState,
    onConnect: () -> Unit,
    onDisconnect: () -> Unit,
) {
    val isConnected    = vpnState is VpnState.Connected
    val isConnecting   = vpnState is VpnState.Connecting || vpnState is VpnState.Reconnecting

    // 600ms pulse on the border when connecting or reconnecting.
    val infiniteTransition = rememberInfiniteTransition(label = "button_pulse")
    val pulseAlpha by infiniteTransition.animateFloat(
        initialValue  = 0.4f,
        targetValue   = 1.0f,
        animationSpec = infiniteRepeatable(
            animation  = tween(600, easing = EaseInOut),
            repeatMode = RepeatMode.Reverse,
        ),
        label = "pulse",
    )

    val borderColor: Color = when {
        isConnected  -> TyraxColors.Red
        isConnecting -> TyraxColors.Red
        else         -> TyraxColors.White
    }
    val borderAlpha = when {
        isConnecting -> pulseAlpha
        else         -> 1f
    }

    val labelText = when {
        isConnecting -> stringResource(R.string.status_breaching_short)
        isConnected  -> stringResource(R.string.btn_disconnect)
        else         -> stringResource(R.string.btn_enter)
    }
    val labelColor = when {
        isConnected -> TyraxColors.Red
        else        -> TyraxColors.White
    }

    Box(
        contentAlignment = Alignment.Center,
        modifier = Modifier
            .size(200.dp)
            .alpha(borderAlpha)
            .border(1.5.dp, borderColor)
            .background(Color.Transparent)
            .clickable {
                if (isConnected) onDisconnect() else onConnect()
            },
    ) {
        Text(
            text      = labelText,
            style     = TyraxTypography.display,
            color     = labelColor,
            textAlign = TextAlign.Center,
        )
    }
}
