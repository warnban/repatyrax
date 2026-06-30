package com.tyrax.domain.usecase

import android.content.Context
import com.tyrax.data.vpn.TyraxVpnManager
import com.tyrax.data.vpn.VpnStateBus
import com.tyrax.data.vpn.XrayManager
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject

/**
 * Resumes a pending tunnel connection after the user grants VPN consent
 * (i.e. after the system dialog raised by [VpnState.NeedsPermission] returns OK).
 *
 * Routes to whichever engine raised the consent request, so the VLESS (Xray) path
 * resumes correctly instead of falling through to WireGuard.
 */
class ResumeConnectionUseCase @Inject constructor(
    @ApplicationContext private val context: Context,
) {
    operator fun invoke() = when (VpnStateBus.activeEngine) {
        VpnStateBus.Engine.XRAY -> XrayManager.retryAfterPermission(context)
        else -> TyraxVpnManager.retryAfterPermission(context)
    }
}
