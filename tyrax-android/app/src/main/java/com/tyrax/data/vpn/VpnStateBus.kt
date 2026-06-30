package com.tyrax.data.vpn

import com.tyrax.domain.model.VpnState
import kotlinx.coroutines.flow.MutableStateFlow

/**
 * Single source of truth for the current tunnel state, shared by both data-plane
 * engines:
 *   - [TyraxVpnManager] (WireGuard / wg-go)
 *   - [XrayManager]     (VLESS + Reality / Xray-core)
 *
 * Only one engine is ever active at a time. Centralising the flow here means the
 * UI (via VpnRepository.vpnState) keeps working no matter which protocol the
 * selected node uses, without merging two competing StateFlows.
 */
internal object VpnStateBus {
    enum class Engine { NONE, WIREGUARD, XRAY }

    val state = MutableStateFlow<VpnState>(VpnState.Disconnected)

    /**
     * Which engine owns the current (or pending) connection. Set by each manager's
     * connect(); used to route consent-resume and disconnect to the right engine.
     */
    @Volatile
    var activeEngine: Engine = Engine.NONE
}
