package com.tyrax.presentation.screens.auth

import android.content.Intent
import android.net.Uri
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.tyrax.domain.repository.AuthRepository
import com.tyrax.domain.usecase.LoginUseCase
import com.tyrax.domain.usecase.RegisterUseCase
import dagger.hilt.android.lifecycle.HiltViewModel
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
                    _uiState.value = AuthUiState.Error(error.message ?: "INVALID CREDENTIALS")
                }
        }
    }

    fun register(email: String, password: String) {
        if (!validateInputs(email, password)) return
        viewModelScope.launch {
            _uiState.value = AuthUiState.Loading
            registerUseCase(email, password)
                .onSuccess { authData ->
                    authRepository.saveToken(authData.token)
                    _uiState.value = AuthUiState.Success(authData.token)
                    _events.send(AuthUiEvent.NavigateToMain)
                }
                .onFailure { error ->
                    _uiState.value = AuthUiState.Error(error.message ?: "REGISTRATION FAILED")
                }
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

    // Polls every 2 seconds for up to 30 seconds after the Telegram bot flow starts.
    private fun pollTelegramStatus(initToken: String) {
        viewModelScope.launch {
            val maxAttempts = 15
            repeat(maxAttempts) { attempt ->
                delay(2_000)
                authRepository.pollTelegramStatus(initToken)
                    .onSuccess { authData ->
                        authRepository.saveToken(authData.token)
                        _uiState.value = AuthUiState.Success(authData.token)
                        _events.send(AuthUiEvent.NavigateToMain)
                        return@launch
                    }
                if (attempt == maxAttempts - 1) {
                    _uiState.value = AuthUiState.Error("CONNECTION FAILED. RETRY.")
                }
            }
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
}
