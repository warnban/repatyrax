package com.tyrax.domain.usecase

import android.content.Context
import com.tyrax.data.local.TokenStore
import com.tyrax.data.vpn.TyraxVpnManager
import com.tyrax.domain.model.Node
import com.tyrax.domain.repository.VpnRepository
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject

/**
 * Provisions a device config (WireGuard) for the target node and hands it to the VPN service.
 * The backend selects the best node and returns its WireGuard config in the device payload.
 *
 * Device name is derived from Android ID ("android-<first 8 chars>") and persisted in
 * DataStore so that the same name is reused across reconnects and app reinstalls.
 * The backend deduplicates by name, so reinstalling simply re-issues the keypair without
 * consuming an extra device slot.
 */
class ConnectToNodeUseCase @Inject constructor(
    @ApplicationContext private val context: Context,
    private val vpnRepository: VpnRepository,
    private val tokenStore: TokenStore,
) {
    suspend operator fun invoke(node: Node? = null): Result<Unit> {
        val deviceName = tokenStore.getOrCreateDeviceName()
        return vpnRepository.addDevice(name = deviceName).mapCatching { deviceConfig ->
            val conf = deviceConfig.wireGuardConf
                ?: throw IllegalStateException("NODE UNAVAILABLE")
            val codename = node?.codename
                ?: deviceConfig.nodes.firstOrNull()?.codename
                ?: "—"
            TyraxVpnManager.connect(context, conf, codename)
        }
    }
}
