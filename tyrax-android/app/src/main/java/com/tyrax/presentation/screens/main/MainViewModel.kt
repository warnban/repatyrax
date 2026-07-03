package com.tyrax.presentation.screens.main

import android.util.Log
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.tyrax.data.local.TokenStore
import com.tyrax.data.local.UpdatePrefs
import com.tyrax.data.vpn.TunnelStatsBus
import com.tyrax.domain.model.UpdateInfo
import com.tyrax.domain.model.VpnState
import com.tyrax.domain.repository.VpnRepository
import com.tyrax.domain.usecase.AddDeviceUseCase
import com.tyrax.domain.usecase.CheckUpdateUseCase
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
    // Subscription info for FREE-tier traffic counter (server-reported usage).
    val tier: String = "FREE",
    val usedBytes: Long = 0,
    val trafficLimitBytes: Long = 3L * 1024 * 1024 * 1024,  // 3 GB default for FREE
    // Paid tiers report unlimited → the UI shows ∞ instead of a metered bar.
    val unlimited: Boolean = false,
    // True while the FREE tunnel is locked out for exceeding the 3 GB quota.
    val trafficBlocked: Boolean = false,
    // True when this device could not be registered because the tier's device
    // limit is exhausted — the UI prompts the user to upgrade.
    val deviceLimitReached: Boolean = false,
    // Transient prompt shown when the user taps ENTER while quota-blocked.
    val trafficBlockedPrompt: Boolean = false,
    // A newer APK the user should install (null = up-to-date / dismissed).
    val updateInfo: UpdateInfo? = null,
)

@HiltViewModel
class MainViewModel @Inject constructor(
    private val connectionSupervisor: ConnectionSupervisor,
    private val getSubscriptionUseCase: GetSubscriptionUseCase,
    private val resumeConnectionUseCase: ResumeConnectionUseCase,
    private val addDeviceUseCase: AddDeviceUseCase,
    private val tokenStore: TokenStore,
    private val vpnRepository: VpnRepository,
    private val checkUpdateUseCase: CheckUpdateUseCase,
    private val updatePrefs: UpdatePrefs,
) : ViewModel() {

    // Snapshot of the server-side subscription used to drive the traffic counter
    // and quota gate. Refreshed on launch and whenever the tunnel goes idle.
    private data class SubInfo(
        val tier: String = "FREE",
        val usedBytes: Long = 0,
        val limitBytes: Long = 3L * 1024 * 1024 * 1024,
        val unlimited: Boolean = false,
        val blocked: Boolean = false,
    )

    private val _sub = MutableStateFlow(SubInfo())
    private val _deviceLimitReached = MutableStateFlow(false)
    private val _trafficBlockedPrompt = MutableStateFlow(false)
    private val _updateInfo = MutableStateFlow<UpdateInfo?>(null)

    init {
        refreshSubscription()
        ensureDeviceRegistered()
        checkForUpdate()
        // Re-read usage whenever the tunnel returns to idle so the counter and
        // the quota gate reflect the traffic just consumed.
        viewModelScope.launch {
            vpnRepository.vpnState.collect { state ->
                if (state is VpnState.Disconnected || state is VpnState.Error) {
                    refreshSubscription()
                }
            }
        }
    }

    private fun refreshSubscription() {
        viewModelScope.launch {
            getSubscriptionUseCase().onSuccess { sub ->
                _sub.value = SubInfo(
                    tier      = sub.tier,
                    usedBytes = sub.usedBytes,
                    limitBytes = if (sub.limitBytes > 0) sub.limitBytes else _sub.value.limitBytes,
                    unlimited = sub.unlimited,
                    blocked   = sub.isBlocked,
                )
            }
        }
    }

    /**
     * Registers THIS device automatically the first time the user lands here on a
     * new install. If the account is already at its tier's device limit, flags the
     * UI to prompt an upgrade. Purely account-side — never touches the tunnel.
     */
    private fun ensureDeviceRegistered() {
        viewModelScope.launch {
            val name = tokenStore.getOrCreateDeviceName()
            val devices = vpnRepository.getDevices().getOrNull() ?: return@launch
            if (devices.any { it.name == name }) return@launch
            addDeviceUseCase(name).onFailure { e ->
                if (e.message?.contains("LIMIT", ignoreCase = true) == true) {
                    _deviceLimitReached.value = true
                }
            }
        }
    }

    private fun checkForUpdate() {
        viewModelScope.launch {
            _updateInfo.value = checkUpdateUseCase()
        }
    }

    /** "ПОЗЖЕ": remember the dismissed version so the banner stops nagging, and hide it. */
    fun onUpdateLater() {
        val info = _updateInfo.value ?: return
        viewModelScope.launch {
            updatePrefs.setDismissed(info.versionCode)
            _updateInfo.value = null
        }
    }

    fun dismissDeviceLimit() {
        _deviceLimitReached.value = false
    }

    fun dismissTrafficBlock() {
        _trafficBlockedPrompt.value = false
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

    // Transient UI flags folded together so the main combine stays within arity.
    private data class Flags(
        val limitReached: Boolean,
        val blockedPrompt: Boolean,
        val update: UpdateInfo?,
    )

    private val _flags = combine(_deviceLimitReached, _trafficBlockedPrompt, _updateInfo) { limit, prompt, update ->
        Flags(limit, prompt, update)
    }

    // Live ping/throughput is merged here so it updates the UI WITHOUT creating a
    // new VpnState instance (which would re-trigger the connection glitch animation).
    val uiState: StateFlow<MainUiState> =
        combine(_vpnBase, _sub, TunnelStatsBus.stats, _flags) { base, sub, stats, flags ->
            val connected = base.vpnState is VpnState.Connected
            base.copy(
                tier                 = sub.tier,
                usedBytes            = sub.usedBytes,
                trafficLimitBytes    = sub.limitBytes,
                unlimited            = sub.unlimited,
                trafficBlocked       = sub.blocked,
                pingMs               = if (connected) stats.pingMs else base.pingMs,
                downBps              = if (connected) stats.downBps else 0,
                upBps                = if (connected) stats.upBps else 0,
                deviceLimitReached   = flags.limitReached,
                trafficBlockedPrompt = flags.blockedPrompt,
                updateInfo           = flags.update,
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
        // FREE quota gate: if the account is locked out, prompt an upgrade instead
        // of starting the tunnel. Purely a UI gate — the supervisor is untouched.
        if (_sub.value.blocked) {
            Log.d(TAG, "connect() blocked — FREE quota exhausted")
            _trafficBlockedPrompt.value = true
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
