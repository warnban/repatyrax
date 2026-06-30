package com.tyrax.presentation.screens.main

import android.app.Activity
import android.widget.Toast
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.core.EaseInOut
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
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
import androidx.compose.runtime.key
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
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
import com.v2ray.ang.service.TProxyService
import kotlinx.coroutines.delay

/** Bump on every build so the device can confirm which APK is actually installed. */
private const val BUILD_TAG = "BUILD 0726-L"

/**
 * Copies a compact Xray diagnostic bundle (running config + error log + recent
 * access log) to the clipboard. Tapping [BUILD_TAG] invokes this so logs can be
 * pasted into chat without adb or file-manager access to Android/data.
 */
private fun copyDiagnostics(ctx: android.content.Context) {
    val dir = ctx.getExternalFilesDir(null) ?: ctx.filesDir
    fun read(name: String, lastLines: Int? = null): String {
        val f = java.io.File(dir, name)
        if (!f.exists()) return "<$name: missing>"
        val text = runCatching { f.readText() }.getOrElse { "<$name: ${it.message}>" }
        return if (lastLines != null) text.lines().takeLast(lastLines).joinToString("\n") else text
    }
    // Drop the UDP teardown spam ("closed pipe" / "use of closed network connection")
    // that floods the error log on disconnect, so Reality/dial errors stay visible.
    fun readErr(lastLines: Int): String {
        val f = java.io.File(dir, "xray_error.log")
        if (!f.exists()) return "<xray_error.log: missing>"
        val kept = runCatching { f.readLines() }.getOrElse { return "<xray_error.log: ${it.message}>" }
            .filterNot {
                val l = it.lowercase()
                l.contains("closed pipe") || l.contains("use of closed network connection")
            }
        return kept.takeLast(lastLines).joinToString("\n")
    }
    val hevStats = runCatching {
        val s = TProxyService.TProxyGetStats()
        if (s.size >= 2) "hev tx=${s[0]} rx=${s[1]} (rx=0 → return path broken)"
        else "hev stats: unexpected length ${s.size}"
    }.getOrElse { "hev stats: ${it.message}" }
    val bundle = buildString {
        appendLine("=== TYRAX DIAG $BUILD_TAG ===")
        appendLine("--- hev tunnel ---")
        appendLine(hevStats)
        appendLine("--- xray_config.json ---")
        appendLine(read("xray_config.json"))
        appendLine("--- xray_error.log (filtered) ---")
        appendLine(readErr(150))
        appendLine("--- xray_access.log (last 30) ---")
        appendLine(read("xray_access.log", 30))
    }
    val clipboard = ctx.getSystemService(android.content.Context.CLIPBOARD_SERVICE) as android.content.ClipboardManager
    clipboard.setPrimaryClip(android.content.ClipData.newPlainText("tyrax-diag", bundle))
    Toast.makeText(ctx, "DIAG COPIED — PASTE TO CHAT", Toast.LENGTH_LONG).show()
}

@Composable
fun MainScreen(
    onNavigateToNodes: () -> Unit,
    onNavigateToSubscription: () -> Unit,
    onNavigateToSettings: () -> Unit = {},
    viewModel: MainViewModel = hiltViewModel(),
) {
    // Hoist context to top of composable — do NOT read LocalContext inside Column/Box bodies.
    val ctx = LocalContext.current
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
                Spacer(modifier = Modifier.height(8.dp))
                Text(
                    text  = BUILD_TAG,
                    style = TyraxTypography.label.copy(color = TyraxColors.Red),
                    modifier = Modifier.clickable { copyDiagnostics(ctx) },
                )
            }

            // ── CENTER ZONE — 200×200dp main button ───────────────────────────
            MainButton(
                vpnState     = uiState.vpnState,
                onConnect    = { viewModel.connect() },
                onDisconnect = { viewModel.disconnect() },
                onTapDebug   = { Toast.makeText(ctx, "TYRAX TAP", Toast.LENGTH_SHORT).show() },
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
    onTapDebug: () -> Unit = {},
) {
    val isConnected  = vpnState is VpnState.Connected
    val isConnecting = vpnState is VpnState.Connecting || vpnState is VpnState.Reconnecting

    // 600ms pulse on the border while connecting.
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

    val borderColor: Color = if (isConnected || isConnecting) TyraxColors.Red else TyraxColors.White
    val borderAlpha        = if (isConnecting) pulseAlpha else 1f

    val labelText = when {
        isConnecting -> stringResource(R.string.status_breaching_short)
        isConnected  -> stringResource(R.string.btn_disconnect)
        else         -> stringResource(R.string.btn_enter)
    }
    val labelColor = if (isConnected) TyraxColors.Red else TyraxColors.White

    val interactionSource = remember { MutableInteractionSource() }
    Box(
        contentAlignment = Alignment.Center,
        modifier = Modifier
            .size(200.dp)
            // clickable BEFORE alpha/border — alpha() creates a graphics layer and must
            // not wrap the gesture recogniser or hit-testing can silently fail.
            .clickable(
                interactionSource = interactionSource,
                indication        = null,
            ) {
                android.util.Log.d("TYRAX-UI", "button tapped isConnected=$isConnected")
                onTapDebug()
                if (isConnected) onDisconnect() else onConnect()
            }
            .alpha(borderAlpha)
            .border(1.5.dp, borderColor),
    ) {
        Text(
            text      = labelText,
            style     = TyraxTypography.display,
            color     = labelColor,
            textAlign = TextAlign.Center,
        )
    }
}
