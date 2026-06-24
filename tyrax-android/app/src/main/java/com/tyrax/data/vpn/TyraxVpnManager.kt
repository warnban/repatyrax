package com.tyrax.data.vpn

import android.content.Context
import android.net.VpnService
import com.tyrax.domain.model.VpnState
import com.wireguard.android.backend.Backend
import com.wireguard.android.backend.GoBackend
import com.wireguard.android.backend.Tunnel
import com.wireguard.config.Config
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.BufferedReader
import java.io.StringReader
import java.net.InetAddress

/**
 * Drives the WireGuard data plane via the userspace wg-go backend.
 *
 * The `com.wireguard.android:tunnel` library owns the actual [android.net.VpnService]
 * (the bundled GoBackend.VpnService, declared in the manifest) and builds the tun
 * interface itself from the parsed [Config] — there is no way to inject an externally
 * built tun fd. We therefore drive it through [Backend.setState]:
 *   - UP   → wg-go turns on, packets flow.
 *   - DOWN → wg-go tears down.
 *
 * Consent: [VpnService.prepare] must return null before bringing the tunnel UP. When
 * it returns an Intent we surface [VpnState.NeedsPermission] so the UI can launch the
 * system consent dialog, then call [retryAfterPermission].
 */
object TyraxVpnManager {

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val lock = Any()

    @Volatile
    private var backend: GoBackend? = null
    private var statsJob: Job? = null

    // Retained so the connection can resume after the consent dialog returns.
    private var pendingConfig: String? = null
    private var pendingCodename: String = "—"
    private var currentEndpointHost: String? = null

    private val _state = MutableStateFlow<VpnState>(VpnState.Disconnected)
    val state: StateFlow<VpnState> = _state.asStateFlow()

    // The active tunnel handle. wg-go puts a Tunnel into a terminal state once it
    // has been brought DOWN, so the instance cannot be reused — we create a fresh
    // one for every connect and drop the reference on disconnect.
    @Volatile
    private var tunnel: Tunnel? = null

    // wg-go identifies the interface by this name (<=15 chars).
    private fun newTunnel(): Tunnel = object : Tunnel {
        override fun getName(): String = "tyrax"
        override fun onStateChange(newState: Tunnel.State) {
            // Reflect an unsolicited drop (e.g. OS revoke) in the shared state.
            if (newState == Tunnel.State.DOWN && _state.value is VpnState.Connected) {
                statsJob?.cancel()
                tunnel = null
                _state.value = VpnState.Disconnected
            }
        }
    }

    private fun backend(context: Context): GoBackend =
        backend ?: synchronized(lock) {
            backend ?: GoBackend(context.applicationContext).also { backend = it }
        }

    /**
     * Provisions consent then brings the tunnel UP. If VPN consent is missing,
     * emits [VpnState.NeedsPermission] and returns without connecting.
     */
    fun connect(context: Context, wireGuardConf: String, nodeCodename: String) {
        pendingConfig = wireGuardConf
        pendingCodename = nodeCodename

        val consent = VpnService.prepare(context.applicationContext)
        if (consent != null) {
            _state.value = VpnState.NeedsPermission(consent)
            return
        }
        bringUp(context.applicationContext, wireGuardConf, nodeCodename)
    }

    /** Resumes the pending connection after the user grants VPN consent. */
    fun retryAfterPermission(context: Context) {
        val conf = pendingConfig ?: return
        bringUp(context.applicationContext, conf, pendingCodename)
    }

    private fun bringUp(appContext: Context, conf: String, codename: String) {
        _state.value = VpnState.Connecting
        scope.launch {
            try {
                val be = backend(appContext)
                val config = Config.parse(BufferedReader(StringReader(conf)))
                currentEndpointHost = config.peers.firstOrNull()
                    ?.endpoint?.orElse(null)?.host

                // Fresh tunnel every time: a DOWN'd tunnel can't be brought UP again.
                val t = newTunnel().also { tunnel = it }
                be.setState(t, Tunnel.State.UP, config)

                _state.value = VpnState.Connected(
                    nodeCodename = codename,
                    protocol = "wireguard",
                    pingMs = 0,
                )
                startStatsPolling(be, t)
            } catch (e: Exception) {
                _state.value = VpnState.Error(e.message ?: "CONNECTION FAILED. NODE UNAVAILABLE.")
            }
        }
    }

    fun disconnect(context: Context) {
        statsJob?.cancel()
        statsJob = null
        val appContext = context.applicationContext
        val active = tunnel
        tunnel = null
        scope.launch {
            if (active != null) {
                runCatching { backend(appContext).setState(active, Tunnel.State.DOWN, null) }
            }
            pendingConfig = null
            currentEndpointHost = null
            _state.value = VpnState.Disconnected
        }
    }

    // ── Live throughput counters ────────────────────────────────────────────────
    // Polls wg-go every 3s and folds total Rx/Tx (and a reachability ping) into the
    // Connected state so MainViewModel can render the FREE-tier traffic counter.
    private fun startStatsPolling(be: Backend, t: Tunnel) {
        statsJob?.cancel()
        statsJob = scope.launch {
            while (isActive) {
                delay(3_000)
                runCatching {
                    val stats = be.getStatistics(t)
                    val ping = currentEndpointHost?.let { measurePing(it) } ?: 0L
                    val current = _state.value
                    if (current is VpnState.Connected) {
                        _state.value = current.copy(
                            bytesIn = stats.totalRx(),
                            bytesOut = stats.totalTx(),
                            pingMs = ping.toInt().coerceAtLeast(0),
                        )
                    }
                }
            }
        }
    }

    /** ICMP reachability probe; returns round-trip ms, or -1 if unreachable. */
    private suspend fun measurePing(host: String): Long = withContext(Dispatchers.IO) {
        try {
            val address = InetAddress.getByName(host)
            val start = System.currentTimeMillis()
            if (address.isReachable(2_000)) System.currentTimeMillis() - start else -1L
        } catch (_: Exception) {
            -1L
        }
    }
}
