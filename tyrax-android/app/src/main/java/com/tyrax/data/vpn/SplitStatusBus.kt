package com.tyrax.data.vpn

import kotlinx.coroutines.flow.MutableStateFlow

/**
 * Live, read-only status of the RU split-tunnel for the UI (CONTROL screen).
 * Filled by [SplitDiagnostics]; kept separate from [VpnStateBus] so status updates
 * never re-fire the main-screen connection animation.
 */
object SplitStatusBus {

    data class SplitStatus(
        /** Number of RU services/domains currently routed directly (bypassing the tunnel). */
        val bypassCount: Int = 0,
        /** elapsedRealtime() of the last diagnostic sweep, or 0 if none yet. */
        val lastCheckedAt: Long = 0,
        /** The most recent domain the self-healing loop auto-added, if any. */
        val lastAutoAdded: String? = null,
    )

    val status = MutableStateFlow(SplitStatus())

    fun reset() {
        status.value = SplitStatus()
    }
}
