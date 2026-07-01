package com.tyrax.data.vpn

import kotlinx.coroutines.flow.MutableStateFlow

/**
 * Live, read-only telemetry for the active tunnel: latency and instantaneous
 * throughput. Kept SEPARATE from [VpnStateBus]/[VpnState] on purpose — these
 * values change every second, and folding them into the VpnState.Connected object
 * would re-fire the connection animation on the main screen. The UI observes this
 * flow independently of the connection state.
 *
 * This bus is filled by a passive poller that only READS counters
 * (TProxyGetStats) and probes latency; it never touches the tunnel lifecycle.
 */
object TunnelStatsBus {

    data class Stats(
        val pingMs: Int = 0,
        /** Download rate in bytes per second (traffic coming back to the device). */
        val downBps: Long = 0,
        /** Upload rate in bytes per second. */
        val upBps: Long = 0,
    )

    val stats = MutableStateFlow(Stats())

    fun reset() {
        stats.value = Stats()
    }
}
