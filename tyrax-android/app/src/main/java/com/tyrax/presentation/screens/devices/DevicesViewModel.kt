package com.tyrax.presentation.screens.devices

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.tyrax.domain.model.InviteRecord
import com.tyrax.domain.model.Subscription
import com.tyrax.domain.model.UserDevice
import com.tyrax.domain.usecase.AddDeviceUseCase
import com.tyrax.domain.usecase.DeleteDeviceUseCase
import com.tyrax.domain.usecase.GetSubscriptionUseCase
import com.tyrax.domain.usecase.InviteAccountUseCase
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
    val invites: List<InviteRecord> = emptyList(),
    val error: String? = null,
    val addedBanner: Boolean = false,
    val inviteError: String? = null,
)

@HiltViewModel
class DevicesViewModel @Inject constructor(
    private val getSubscriptionUseCase: GetSubscriptionUseCase,
    private val addDeviceUseCase: AddDeviceUseCase,
    private val deleteDeviceUseCase: DeleteDeviceUseCase,
    private val inviteAccountUseCase: InviteAccountUseCase,
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
                .onSuccess { sub ->
                    _uiState.update { it.copy(isLoading = false, subscription = sub) }
                }
                .onFailure { e ->
                    _uiState.update { it.copy(isLoading = false, error = e.message) }
                }
        }
    }

    fun addDevice() {
        val existing = _uiState.value.devices.size
        val name = "Device ${existing + 1}"
        viewModelScope.launch {
            addDeviceUseCase(name)
                .onSuccess { config ->
                    val newDevice = UserDevice(
                        id        = config.deviceId,
                        name      = name,
                        createdAt = "",
                    )
                    _uiState.update { s ->
                        s.copy(
                            devices    = s.devices + newDevice,
                            addedBanner = true,
                            error      = null,
                        )
                    }
                }
                .onFailure { e ->
                    _uiState.update { it.copy(error = e.message) }
                }
        }
    }

    fun dismissBanner() {
        _uiState.update { it.copy(addedBanner = false) }
    }

    fun deleteDevice(deviceId: String) {
        viewModelScope.launch {
            deleteDeviceUseCase(deviceId)
                .onSuccess {
                    _uiState.update { s ->
                        s.copy(devices = s.devices.filter { d -> d.id != deviceId })
                    }
                }
                .onFailure { e ->
                    _uiState.update { it.copy(error = e.message) }
                }
        }
    }

    fun sendInvite(accountId: String) {
        viewModelScope.launch {
            inviteAccountUseCase.send(accountId)
                .onSuccess { load() }
                .onFailure { e -> _uiState.update { it.copy(inviteError = e.message) } }
        }
    }

    fun removeInvite(accountId: String) {
        viewModelScope.launch {
            inviteAccountUseCase.remove(accountId)
                .onSuccess {
                    _uiState.update { s ->
                        s.copy(invites = s.invites.filter { i -> i.inviteeId != accountId })
                    }
                }
                .onFailure { e -> _uiState.update { it.copy(error = e.message) } }
        }
    }
}
