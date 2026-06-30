package com.tyrax.domain.usecase

import android.content.Context
import com.tyrax.data.vpn.TyraxVpnManager
import com.tyrax.data.vpn.VpnStateBus
import com.tyrax.data.vpn.XrayManager
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject

class DisconnectUseCase @Inject constructor(
    @ApplicationContext private val context: Context,
) {
    operator fun invoke() {
        when (VpnStateBus.activeEngine) {
            VpnStateBus.Engine.XRAY -> XrayManager.disconnect(context)
            else -> TyraxVpnManager.disconnect(context)
        }
    }
}
