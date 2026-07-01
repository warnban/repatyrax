package com.tyrax.presentation.screens.main

import android.util.Log
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.tyrax.data.vpn.TunnelStatsBus
import com.tyrax.domain.model.VpnState
import com.tyrax.domain.repository.VpnRepository
import com.tyrax.domain.usecase.ConnectionSupervisor
import com.tyrax.domain.usecase.GetSubscriptionUseCase
import com.tyrax.domain.usecase.ResumeConnectionUseCase
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.scan
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
import javax.inject.Inject

data class MainUiState(
    val vpnState: VpnState = VpnState.Disconnected,
    val currentNode: String = "",
    val pingMs: Int = 0,
    val trafficIn: Long = 0,
    val trafficOut: Long = 0,
    // Live throughput (bytes/sec) from the passive telemetry poller.
    val downBps: Long = 0,
    val upBps: Long = 0,
    // Subscription info for FREE-tier traffic counter.
    val tier: String = "FREE",
    val trafficLimitBytes: Long = 3L * 1024 * 1024 * 1024,  // 3 GB default for FREE
) {
    val trafficUsedBytes: Long get() = trafficIn + trafficOut
}

@HiltViewModel
class MainViewModel @Inject constructor(
    private val connectionSupervisor: ConnectionSupervisor,
    private val getSubscriptionUseCase: GetSubscriptionUseCase,
    private val resumeConnectionUseCase: ResumeConnectionUseCase,
    vpnRepository: VpnRepository,
) : ViewModel() {

    private val _tier = MutableStateFlow("FREE")

    init {
        viewModelScope.launch {
            getSubscriptionUseCase()
                .onSuccess { sub -> _tier.value = sub.tier }
        }
    }

    // scan retains the last-known node name during Reconnecting so the UI never blanks.
    private val _vpnBase: StateFlow<MainUiState> = vpnRepository.vpnState
        .scan(MainUiState()) { prev, vpnState ->
            when (vpnState) {
                is VpnState.Connected -> prev.copy(
                    vpnState    = vpnState,
                    currentNode = vpnState.nodeCodename,
                    pingMs      = vpnState.pingMs,
                    trafficIn   = vpnState.bytesIn,
                    trafficOut  = vpnState.bytesOut,
                )
                is VpnState.Reconnecting -> prev.copy(vpnState = vpnState)
                else -> prev.copy(vpnState = vpnState)
            }
        }
        .stateIn(viewModelScope, SharingStarted.Eagerly, MainUiState())

    // Live ping/throughput is merged here so it updates the UI WITHOUT creating a
    // new VpnState instance (which would re-trigger the connection glitch animation).
    val uiState: StateFlow<MainUiState> =
        combine(_vpnBase, _tier, TunnelStatsBus.stats) { base, tier, stats ->
            val connected = base.vpnState is VpnState.Connected
            base.copy(
                tier    = tier,
                pingMs  = if (connected) stats.pingMs else base.pingMs,
                downBps = if (connected) stats.downBps else 0,
                upBps   = if (connected) stats.upBps else 0,
            )
        }.stateIn(
            scope        = viewModelScope,
            started      = SharingStarted.WhileSubscribed(5_000),
            initialValue = MainUiState(),
        )

    fun connect() {
        Log.d(TAG, "connect() called, state=${uiState.value.vpnState}")
        // Guard: never start while already connecting or connected.
        val state = uiState.value.vpnState
        if (state is VpnState.Connecting || state is VpnState.Connected) {
            Log.d(TAG, "connect() ignored — state=${state::class.simpleName}")
            return
        }
        // The supervisor owns node selection, health monitoring and silent
        // failover; it is idempotent and runs in an app-scoped coroutine.
        connectionSupervisor.start()
    }

    fun disconnect() {
        connectionSupervisor.stop()
    }

    /** Called by the UI once the system VPN consent dialog returns OK. */
    fun onPermissionGranted() {
        Log.d(TAG, "onPermissionGranted()")
        resumeConnectionUseCase()
    }

    companion object {
        private const val TAG = "TYRAX-VM"
    }
}
