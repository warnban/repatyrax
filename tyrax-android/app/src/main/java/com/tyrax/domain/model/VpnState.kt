package com.tyrax.domain.model

import android.content.Intent

sealed class VpnState {
    object Disconnected : VpnState()
    object Connecting : VpnState()
    data class Connected(
        val nodeCodename: String,
        val protocol: String,
        val pingMs: Int,
        val bytesIn: Long = 0,
        val bytesOut: Long = 0,
    ) : VpnState()
    data class Error(val message: String) : VpnState()
    object Reconnecting : VpnState()

    /**
     * The OS has not yet granted VPN consent. [intent] must be launched from an
     * Activity via startActivityForResult; on RESULT_OK the connection resumes.
     */
    data class NeedsPermission(val intent: Intent) : VpnState()
}
