package com.tyrax.data.vpn

import android.content.Context
import android.content.Intent
import android.net.VpnService
import android.util.Log
import androidx.core.content.ContextCompat
import com.tyrax.domain.model.VpnState
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

/**
 * VLESS + Reality data plane (Xray-core). Mirrors [TyraxVpnManager]'s surface so
 * callers can branch on protocol without caring which engine is underneath:
 *   - [state], [connect], [disconnect], [retryAfterPermission].
 *
 * The actual engine (libv2ray CoreController + tun2socks) lives in
 * [TyraxXrayVpnService]; this object only handles consent and lifecycle intents,
 * so it has no compile-time dependency on the libv2ray AAR.
 */
object XrayManager {

    private const val TAG = "TYRAX-Xray"

    val state: StateFlow<VpnState> = VpnStateBus.state.asStateFlow()
    private val _state get() = VpnStateBus.state

    // Retained so the connection can resume after the consent dialog returns.
    private var pendingConfig: String? = null
    private var pendingCodename: String = "—"
    private var pendingSplitEnabled: Boolean = false
    private var pendingBypassDomains: List<String> = emptyList()
    private var pendingBypassApps: List<String> = emptyList()

    /**
     * Starts the Xray VpnService with the ready-to-use config JSON from POST /vpn/connect,
     * carrying the RU split-tunnel config so RU services bypass the tunnel.
     */
    fun startVpn(
        context: Context,
        xrayConfigJson: String,
        nodeCodename: String,
        split: XrayConfigPatcher.SplitConfig = XrayConfigPatcher.SplitConfig.DISABLED,
        bypassApps: List<String> = emptyList(),
    ) {
        Log.d(TAG, "startVpn() node=$nodeCodename configLen=${xrayConfigJson.length} split=${split.enabled}")
        pendingConfig = xrayConfigJson
        pendingCodename = nodeCodename
        pendingSplitEnabled = split.enabled
        pendingBypassDomains = split.bypassDomains
        pendingBypassApps = bypassApps
        VpnStateBus.activeEngine = VpnStateBus.Engine.XRAY

        val consent = VpnService.prepare(context.applicationContext)
        if (consent != null) {
            Log.d(TAG, "startVpn() consent required -> NeedsPermission")
            _state.value = VpnState.NeedsPermission(consent)
            return
        }
        Log.d(TAG, "startVpn() consent already granted -> startService")
        startService(context.applicationContext, xrayConfigJson, nodeCodename)
    }

    /** Resumes the pending connection after the user grants VPN consent. */
    fun retryAfterPermission(context: Context) {
        Log.d(TAG, "retryAfterPermission() pending=${pendingConfig != null}")
        val cfg = pendingConfig ?: return
        startService(context.applicationContext, cfg, pendingCodename)
    }

    fun disconnect(context: Context) {
        Log.d(TAG, "disconnect()")
        val intent = Intent(context.applicationContext, TyraxXrayVpnService::class.java).apply {
            action = TyraxXrayVpnService.ACTION_DISCONNECT
        }
        context.applicationContext.startService(intent)
        pendingConfig = null
        VpnStateBus.activeEngine = VpnStateBus.Engine.NONE
    }

    private fun startService(appContext: Context, xrayConfigJson: String, codename: String) {
        Log.d(TAG, "startService() -> ${TyraxXrayVpnService.ACTION_CONNECT}")
        _state.value = VpnState.Connecting
        val intent = Intent(appContext, TyraxXrayVpnService::class.java).apply {
            action = TyraxXrayVpnService.ACTION_CONNECT
            putExtra(TyraxXrayVpnService.EXTRA_CONFIG_JSON, xrayConfigJson)
            putExtra(TyraxXrayVpnService.EXTRA_CODENAME, codename)
            putExtra(TyraxXrayVpnService.EXTRA_SPLIT_ENABLED, pendingSplitEnabled)
            putStringArrayListExtra(TyraxXrayVpnService.EXTRA_BYPASS_DOMAINS, ArrayList(pendingBypassDomains))
            putStringArrayListExtra(TyraxXrayVpnService.EXTRA_BYPASS_APPS, ArrayList(pendingBypassApps))
        }
        ContextCompat.startForegroundService(appContext, intent)
    }
}
