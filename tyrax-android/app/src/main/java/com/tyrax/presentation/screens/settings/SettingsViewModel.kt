package com.tyrax.presentation.screens.settings

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.tyrax.data.local.TokenStore
import com.tyrax.domain.repository.AuthRepository
import com.tyrax.domain.usecase.GetSubscriptionUseCase
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.firstOrNull
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

data class SettingsUiState(
    val email: String? = null,
    val tier: String? = null,
    val devicesInfo: String? = null,
    val telegramLinked: Boolean = false,
    val loggedOut: Boolean = false,
)

sealed class SettingsUiEvent {
    data class OpenUrl(val url: String) : SettingsUiEvent()
}

@HiltViewModel
class SettingsViewModel @Inject constructor(
    private val authRepository: AuthRepository,
    private val getSubscriptionUseCase: GetSubscriptionUseCase,
    private val tokenStore: TokenStore,
) : ViewModel() {

    private val _uiState = MutableStateFlow(SettingsUiState())
    val uiState: StateFlow<SettingsUiState> = _uiState

    private val _events = Channel<SettingsUiEvent>(Channel.BUFFERED)
    val events = _events.receiveAsFlow()

    init {
        loadProfile()
        viewModelScope.launch {
            getSubscriptionUseCase().onSuccess { sub ->
                _uiState.update {
                    it.copy(
                        tier        = it.tier ?: sub.tier,
                        devicesInfo = "${sub.devicesCount}/${sub.devicesLimit}",
                    )
                }
            }
        }
    }

    private fun loadProfile() {
        viewModelScope.launch {
            val cachedEmail = tokenStore.email.firstOrNull()
            _uiState.update { it.copy(email = it.email ?: cachedEmail) }
            authRepository.getProfile().onSuccess { profile ->
                _uiState.update {
                    it.copy(
                        email          = profile.email ?: it.email,
                        tier           = profile.tier ?: it.tier,
                        telegramLinked = profile.telegramLinked,
                    )
                }
            }
        }
    }

    /** Starts the Telegram link flow: opens the bot, then polls until confirmed. */
    fun linkTelegram() {
        viewModelScope.launch {
            authRepository.getTelegramInitUrl().onSuccess { result ->
                _events.send(SettingsUiEvent.OpenUrl(result.botUrl))
                pollTelegramLink(result.initToken)
            }
        }
    }

    // Polls every 2s for up to 30s; on confirmation, refreshes the linked state.
    private fun pollTelegramLink(initToken: String) {
        viewModelScope.launch {
            repeat(15) {
                delay(2_000)
                val confirmed = authRepository.pollTelegramStatus(initToken).getOrNull()
                if (confirmed != null) {
                    authRepository.saveToken(confirmed.token)
                    loadProfile()
                    return@launch
                }
            }
        }
    }

    fun logout() {
        viewModelScope.launch {
            authRepository.logout()
            _uiState.update { it.copy(loggedOut = true) }
        }
    }

    // ── About / legal ────────────────────────────────────────────────────────
    fun openPrivacyPolicy() = openUrl(PRIVACY_URL)
    fun openTerms() = openUrl(TERMS_URL)
    fun openSupport() = openUrl(SUPPORT_URL)

    private fun openUrl(url: String) {
        viewModelScope.launch { _events.send(SettingsUiEvent.OpenUrl(url)) }
    }

    private companion object {
        // TODO: point these at the real hosted documents before release.
        const val PRIVACY_URL = "https://tyrax.app/privacy"
        const val TERMS_URL = "https://tyrax.app/terms"
        const val SUPPORT_URL = "https://t.me/tyraxvpnbot"
    }
}
