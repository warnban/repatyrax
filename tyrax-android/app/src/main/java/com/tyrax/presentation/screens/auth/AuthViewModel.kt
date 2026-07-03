package com.tyrax.presentation.screens.auth

import android.content.Intent
import android.net.Uri
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.tyrax.domain.repository.AuthRepository
import com.tyrax.domain.usecase.LoginUseCase
import com.tyrax.domain.usecase.RegisterUseCase
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.Job
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

sealed class AuthUiState {
    object Idle : AuthUiState()
    object Loading : AuthUiState()
    data class Success(val token: String) : AuthUiState()
    data class Error(val message: String) : AuthUiState()
}

sealed class AuthUiEvent {
    object NavigateToMain : AuthUiEvent()
    data class NavigateToVerify(val email: String) : AuthUiEvent()
    data class OpenUrl(val url: String) : AuthUiEvent()
}

@HiltViewModel
class AuthViewModel @Inject constructor(
    private val loginUseCase: LoginUseCase,
    private val registerUseCase: RegisterUseCase,
    private val authRepository: AuthRepository,
) : ViewModel() {

    private val _uiState = MutableStateFlow<AuthUiState>(AuthUiState.Idle)
    val uiState: StateFlow<AuthUiState> = _uiState

    private val _events = Channel<AuthUiEvent>(Channel.BUFFERED)
    val events = _events.receiveAsFlow()

    private var telegramPollJob: Job? = null

    fun login(email: String, password: String) {
        if (!validateInputs(email, password)) return
        viewModelScope.launch {
            _uiState.value = AuthUiState.Loading
            loginUseCase(email, password)
                .onSuccess { authData ->
                    authRepository.saveToken(authData.token)
                    _uiState.value = AuthUiState.Success(authData.token)
                    _events.send(AuthUiEvent.NavigateToMain)
                }
                .onFailure { error ->
                    val message = error.message ?: "INVALID CREDENTIALS"
                    // An unconfirmed identity is routed to the verify screen (the
                    // backend has already re-sent a fresh code) instead of a dead end.
                    if (message.contains("NOT CONFIRMED", ignoreCase = true)) {
                        _uiState.value = AuthUiState.Idle
                        _events.send(AuthUiEvent.NavigateToVerify(email))
                    } else {
                        _uiState.value = AuthUiState.Error(message)
                    }
                }
        }
    }

    fun register(email: String, password: String) {
        if (!validateInputs(email, password)) return
        viewModelScope.launch {
            _uiState.value = AuthUiState.Loading
            registerUseCase(email, password)
                .onSuccess { authData ->
                    // Hard gate: no session until the email is confirmed.
                    if (authData.verificationRequired) {
                        _uiState.value = AuthUiState.Idle
                        _events.send(AuthUiEvent.NavigateToVerify(email))
                    } else {
                        authRepository.saveToken(authData.token)
                        _uiState.value = AuthUiState.Success(authData.token)
                        _events.send(AuthUiEvent.NavigateToMain)
                    }
                }
                .onFailure { error ->
                    _uiState.value = AuthUiState.Error(error.message ?: "REGISTRATION FAILED")
                }
        }
    }

    /** Confirms the 6-digit code and, on success, opens a session. */
    fun verify(email: String, code: String) {
        if (email.isBlank() || code.isBlank()) {
            _uiState.value = AuthUiState.Error("INVALID CODE")
            return
        }
        viewModelScope.launch {
            _uiState.value = AuthUiState.Loading
            authRepository.verifyEmail(email, code)
                .onSuccess { authData ->
                    authRepository.saveToken(authData.token)
                    _uiState.value = AuthUiState.Success(authData.token)
                    _events.send(AuthUiEvent.NavigateToMain)
                }
                .onFailure { error ->
                    _uiState.value = AuthUiState.Error(error.message ?: "INVALID OR EXPIRED CODE")
                }
        }
    }

    /** Requests a fresh confirmation code. Silent — the screen shows its own hint. */
    fun resend(email: String) {
        viewModelScope.launch {
            authRepository.resendVerification(email)
        }
    }

    fun startTelegramAuth() {
        viewModelScope.launch {
            _uiState.value = AuthUiState.Loading
            authRepository.getTelegramInitUrl()
                .onSuccess { result ->
                    _events.send(AuthUiEvent.OpenUrl(result.botUrl))
                    pollTelegramStatus(result.initToken)
                }
                .onFailure { error ->
                    _uiState.value = AuthUiState.Error(error.message ?: "CONNECTION FAILED. RETRY.")
                }
        }
    }

    /**
     * Polls /auth/telegram-status until the bot confirms the user.
     *
     * Rate-limit aware: the endpoint allows 10 req/min. We poll every 8s (7.5/min)
     * and, after 3 consecutive HTTP 429s, back off to 15s. A non-429 outcome resets
     * to the base interval. Polling continues for a full 5-minute budget — long
     * enough for the user to open Telegram, /start the bot, and return.
     */
    private fun pollTelegramStatus(initToken: String) {
        telegramPollJob?.cancel()
        telegramPollJob = viewModelScope.launch {
            val totalBudgetMs = 5 * 60 * 1_000L
            val baseIntervalMs = 8_000L
            val backoffIntervalMs = 15_000L
            val backoffAfter = 3

            val startedAt = System.currentTimeMillis()
            var consecutive429 = 0
            var intervalMs = baseIntervalMs

            while (System.currentTimeMillis() - startedAt < totalBudgetMs) {
                delay(intervalMs)
                val result = authRepository.pollTelegramStatus(initToken)
                result.onSuccess { authData ->
                    authRepository.saveToken(authData.token)
                    _uiState.value = AuthUiState.Success(authData.token)
                    _events.send(AuthUiEvent.NavigateToMain)
                    return@launch
                }
                // Still waiting (404/null) or throttled (429). Adjust cadence.
                val message = result.exceptionOrNull()?.message.orEmpty()
                if (message.contains("429") || message.contains("TOO MANY", ignoreCase = true)) {
                    consecutive429++
                    if (consecutive429 >= backoffAfter) intervalMs = backoffIntervalMs
                } else {
                    consecutive429 = 0
                    intervalMs = baseIntervalMs
                }
            }
            _uiState.value = AuthUiState.Error("CONNECTION FAILED. RETRY.")
        }
    }

    fun clearError() {
        if (_uiState.value is AuthUiState.Error) {
            _uiState.value = AuthUiState.Idle
        }
    }

    private fun validateInputs(email: String, password: String): Boolean {
        if (email.isBlank() || password.isBlank()) {
            _uiState.value = AuthUiState.Error("INVALID CREDENTIALS")
            return false
        }
        return true
    }

    override fun onCleared() {
        telegramPollJob?.cancel()
        super.onCleared()
    }
}
