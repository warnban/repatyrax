package com.tyrax.presentation.screens.devices

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.tyrax.domain.model.Subscription
import com.tyrax.domain.model.UserDevice
import com.tyrax.domain.repository.VpnRepository
import com.tyrax.domain.usecase.DeleteDeviceUseCase
import com.tyrax.domain.usecase.GetSubscriptionUseCase
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

data class DevicesUiState(
    val isLoading: Boolean = false,
    val subscription: Subscription? = null,
    val devices: List<UserDevice> = emptyList(),
    val error: String? = null,
)

@HiltViewModel
class DevicesViewModel @Inject constructor(
    private val getSubscriptionUseCase: GetSubscriptionUseCase,
    private val deleteDeviceUseCase: DeleteDeviceUseCase,
    private val vpnRepository: VpnRepository,
) : ViewModel() {

    private val _uiState = MutableStateFlow(DevicesUiState())
    val uiState: StateFlow<DevicesUiState> = _uiState

    init {
        load()
    }

    fun load() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
            getSubscriptionUseCase()
                .onSuccess { sub -> _uiState.update { it.copy(subscription = sub) } }
                .onFailure { e -> _uiState.update { it.copy(error = e.message) } }

            vpnRepository.getDevices()
                .onSuccess { devices -> _uiState.update { it.copy(isLoading = false, devices = devices) } }
                .onFailure { e -> _uiState.update { it.copy(isLoading = false, error = e.message) } }
        }
    }

    fun deleteDevice(deviceId: String) {
        viewModelScope.launch {
            deleteDeviceUseCase(deviceId)
                .onSuccess {
                    _uiState.update { s -> s.copy(devices = s.devices.filter { d -> d.id != deviceId }) }
                    // Refresh slot usage after freeing a device.
                    getSubscriptionUseCase().onSuccess { sub ->
                        _uiState.update { it.copy(subscription = sub) }
                    }
                }
                .onFailure { e -> _uiState.update { it.copy(error = e.message) } }
        }
    }
}
