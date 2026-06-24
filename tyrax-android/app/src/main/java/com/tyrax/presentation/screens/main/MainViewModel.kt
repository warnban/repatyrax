package com.tyrax.presentation.screens.main

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.tyrax.domain.model.VpnState
import com.tyrax.domain.repository.VpnRepository
import com.tyrax.domain.usecase.ConnectToNodeUseCase
import com.tyrax.domain.usecase.DisconnectUseCase
import com.tyrax.domain.usecase.GetBestNodeUseCase
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
    // Subscription info for FREE-tier traffic counter.
    val tier: String = "FREE",
    val trafficLimitBytes: Long = 3L * 1024 * 1024 * 1024,  // 3 GB default for FREE
) {
    val trafficUsedBytes: Long get() = trafficIn + trafficOut
}

@HiltViewModel
class MainViewModel @Inject constructor(
    private val connectToNodeUseCase: ConnectToNodeUseCase,
    private val disconnectUseCase: DisconnectUseCase,
    private val getBestNodeUseCase: GetBestNodeUseCase,
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

    val uiState: StateFlow<MainUiState> =
        combine(_vpnBase, _tier) { base, tier ->
            base.copy(tier = tier)
        }.stateIn(
            scope        = viewModelScope,
            started      = SharingStarted.WhileSubscribed(5_000),
            initialValue = MainUiState(),
        )

    fun connect() {
        viewModelScope.launch {
            val best = getBestNodeUseCase().getOrNull()
            connectToNodeUseCase(best)
        }
    }

    fun disconnect() {
        disconnectUseCase()
    }

    /** Called by the UI once the system VPN consent dialog returns OK. */
    fun onPermissionGranted() {
        resumeConnectionUseCase()
    }
}
