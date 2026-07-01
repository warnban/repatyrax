package com.tyrax.domain.usecase

import android.os.SystemClock
import android.util.Log
import com.tyrax.data.vpn.TunnelHealth
import com.tyrax.data.vpn.VpnStateBus
import com.tyrax.domain.model.Node
import com.tyrax.domain.model.NodeStatus
import com.tyrax.domain.model.VpnState
import com.tyrax.domain.repository.VpnRepository
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withTimeoutOrNull
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Keeps the tunnel alive "in any conditions": picks the best node, then watches
 * the live tunnel and, when it dies or gets throttled by DPI, silently switches
 * to the next candidate (a different node / profile) without user interaction.
 *
 * Candidates come from GET /nodes — including different security profiles
 * (Reality-direct, TLS-over-CDN, Vision) provisioned as separate node rows — so
 * profile fallback is just node iteration. Health is measured by probing THROUGH
 * the Xray SOCKS inbound (see [TunnelHealth]).
 *
 * Runs in an app-scoped coroutine so reconnection survives ViewModel recreation.
 */
@Singleton
class ConnectionSupervisor @Inject constructor(
    private val vpnRepository: VpnRepository,
    private val connectToNodeUseCase: ConnectToNodeUseCase,
    private val disconnectUseCase: DisconnectUseCase,
) {

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private var superviseJob: Job? = null

    @Volatile
    private var userWantsConnection = false

    /** User pressed ENTER. Idempotent — a running supervision loop is reused. */
    fun start() {
        if (superviseJob?.isActive == true) {
            Log.d(TAG, "start() ignored — already supervising")
            return
        }
        userWantsConnection = true
        superviseJob = scope.launch { runSupervision() }
    }

    /** User pressed DISCONNECT. Stops supervision and tears the tunnel down. */
    fun stop() {
        userWantsConnection = false
        superviseJob?.cancel()
        superviseJob = null
        disconnectUseCase()
    }

    private suspend fun runSupervision() {
        VpnStateBus.state.value = VpnState.Connecting

        var candidates = loadCandidates()
        if (candidates.isEmpty()) {
            VpnStateBus.state.value = VpnState.Error("NODE UNAVAILABLE")
            return
        }

        var idx = 0
        var consecutiveFailures = 0
        var firstAttempt = true

        while (userWantsConnection && currentCoroutineContext().isActive) {
            val node = candidates[idx % candidates.size]
            Log.d(TAG, "attempt node=${node.codename} (idx=$idx, fails=$consecutiveFailures)")

            val connected = attemptNode(node, firstAttempt)
            firstAttempt = false

            if (!connected) {
                if (!userWantsConnection) break
                consecutiveFailures++
                idx++
                // After cycling through every candidate twice with no luck, back
                // off briefly and refresh the node list (statuses may have changed).
                if (consecutiveFailures >= candidates.size * 2) {
                    VpnStateBus.state.value = VpnState.Reconnecting
                    delay(BACKOFF_MS)
                    consecutiveFailures = 0
                    candidates = loadCandidates().ifEmpty { candidates }
                }
                continue
            }

            // Connected. Watch it until it dies / throttles or the user disconnects.
            consecutiveFailures = 0
            monitorUntilUnhealthy()

            if (!userWantsConnection) break

            // Unhealthy → tear down and switch to the next candidate silently.
            Log.d(TAG, "node=${node.codename} unhealthy → switching")
            VpnStateBus.state.value = VpnState.Reconnecting
            disconnectUseCase()
            delay(SWITCH_DELAY_MS)
            idx++
        }
    }

    /** Loads OPEN nodes ordered by ping; falls back to the single best node. */
    private suspend fun loadCandidates(): List<Node> {
        val nodes = vpnRepository.getNodes().getOrNull()
            ?.filter { it.status == NodeStatus.OPEN }
            ?.sortedBy { it.pingMs }
            ?: emptyList()
        if (nodes.isNotEmpty()) return nodes
        return vpnRepository.getBestNode().getOrNull()?.let { listOf(it) } ?: emptyList()
    }

    /**
     * Starts the engine for [node] and waits until it reports Connected.
     * Treats [VpnState.NeedsPermission] as a pause (the OS consent dialog), not a
     * failure: it keeps waiting for the user to grant consent and resume.
     */
    private suspend fun attemptNode(node: Node, firstAttempt: Boolean): Boolean {
        VpnStateBus.state.value = if (firstAttempt) VpnState.Connecting else VpnState.Reconnecting

        val started = connectToNodeUseCase(node)
        if (started.isFailure) {
            Log.d(TAG, "connectToNode start failed: ${started.exceptionOrNull()?.message}")
            return false
        }

        var deadline = SystemClock.elapsedRealtime() + CONNECT_TIMEOUT_MS
        while (userWantsConnection && currentCoroutineContext().isActive) {
            val remaining = deadline - SystemClock.elapsedRealtime()
            if (remaining <= 0) return false

            val reached = withTimeoutOrNull(remaining) {
                VpnStateBus.state.first {
                    it is VpnState.Connected || it is VpnState.Error || it is VpnState.NeedsPermission
                }
            } ?: return false

            when (reached) {
                is VpnState.Connected -> return true
                is VpnState.Error -> return false
                is VpnState.NeedsPermission -> {
                    // Awaiting the user — wait (generously) for the state to move
                    // on (Connecting/Connected once consent is granted), then loop.
                    deadline = SystemClock.elapsedRealtime() + PERMISSION_WAIT_MS
                    VpnStateBus.state.first { it !is VpnState.NeedsPermission }
                }
                else -> {}
            }
        }
        return false
    }

    /**
     * Returns once the active tunnel is no longer healthy (dead or throttled) or
     * the engine dropped. For the WireGuard engine (no SOCKS inbound) it simply
     * waits for the state to leave Connected.
     */
    private suspend fun monitorUntilUnhealthy() {
        if (VpnStateBus.activeEngine != VpnStateBus.Engine.XRAY) {
            VpnStateBus.state.first { it !is VpnState.Connected }
            return
        }

        delay(INITIAL_GRACE_MS)
        var fails = 0
        while (userWantsConnection && currentCoroutineContext().isActive) {
            if (VpnStateBus.state.value !is VpnState.Connected) return // unsolicited drop

            val result = TunnelHealth.probe()
            when {
                !result.ok -> fails++
                result.elapsedMs > THROTTLE_MS -> fails++ // works but degraded → likely throttled
                else -> fails = 0
            }
            Log.d(TAG, "health ok=${result.ok} took=${result.elapsedMs}ms fails=$fails")

            if (fails >= MAX_FAILS) return
            delay(PROBE_INTERVAL_MS)
        }
    }

    companion object {
        private const val TAG = "TYRAX-Supervisor"

        private const val CONNECT_TIMEOUT_MS = 25_000L
        private const val PERMISSION_WAIT_MS = 120_000L
        // Generous grace + interval so a busy-but-live tunnel (dozens of concurrent
        // background app connections) is never mistaken for dead. Only a sustained
        // run of failures (real node death / hard DPI block) triggers a switch.
        private const val INITIAL_GRACE_MS = 12_000L
        private const val PROBE_INTERVAL_MS = 30_000L
        private const val THROTTLE_MS = 9_000L
        private const val MAX_FAILS = 4
        private const val SWITCH_DELAY_MS = 1_200L
        private const val BACKOFF_MS = 8_000L
    }
}
