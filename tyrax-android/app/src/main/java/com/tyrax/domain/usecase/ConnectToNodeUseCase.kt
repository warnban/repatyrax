package com.tyrax.domain.usecase

import android.content.Context
import android.util.Log
import com.tyrax.data.local.TokenStore
import com.tyrax.data.vpn.TyraxVpnManager
import com.tyrax.data.vpn.XrayManager
import com.tyrax.domain.model.Node
import com.tyrax.domain.repository.VpnRepository
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject

/**
 * Fetches a ready-to-run tunnel config via POST /vpn/connect and hands it to the
 * matching VPN engine:
 *   - "vless"     → Xray-core via [XrayManager.startVpn] (raw Xray JSON in `config`)
 *   - "wireguard" → wg-go via [TyraxVpnManager.connect] (raw WG conf in `config`)
 */
class ConnectToNodeUseCase @Inject constructor(
    @ApplicationContext private val context: Context,
    private val vpnRepository: VpnRepository,
    private val tokenStore: TokenStore,
) {
    suspend operator fun invoke(node: Node? = null): Result<Unit> {
        return try {
            val deviceName = tokenStore.getOrCreateDeviceName()
            val codename = node?.codename
                ?: return Result.failure(IllegalStateException("NODE UNAVAILABLE"))
            Log.d("TYRAX-Connect", "invoke() deviceName=$deviceName preferredNode=$codename")

            val connectResult = vpnRepository.connect(name = deviceName, codename = codename)
            if (connectResult.isFailure) {
                Log.e("TYRAX-Connect", "connectVpn FAILED: ${connectResult.exceptionOrNull()?.message}")
                return connectResult.map { }
            }

            val vpnConfig = connectResult.getOrThrow()

            Log.d(
                "TYRAX-Connect",
                "connectVpn OK protocol=${vpnConfig.protocol} " +
                    "config=${vpnConfig.config.take(50)} codename=$codename",
            )

            when (vpnConfig.protocol) {
                "vless" -> {
                    Log.d("TYRAX-Connect", "vless -> XrayManager.startVpn configLen=${vpnConfig.config.length}")
                    XrayManager.startVpn(context, vpnConfig.config, codename)
                }
                "wireguard" -> {
                    TyraxVpnManager.connect(context, vpnConfig.config, codename)
                }
                else -> throw IllegalStateException("UNSUPPORTED PROTOCOL")
            }
            Result.success(Unit)
        } catch (e: Exception) {
            Log.e("TYRAX-Connect", "EXCEPTION in connect: ${e.message}", e)
            Result.failure(e)
        }
    }
}
