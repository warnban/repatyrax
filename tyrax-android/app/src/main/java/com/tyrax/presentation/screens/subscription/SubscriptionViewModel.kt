package com.tyrax.presentation.screens.subscription

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.tyrax.domain.usecase.GetSubscriptionUseCase
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

data class SubscriptionUiState(
    val isLoading: Boolean = false,
    val currentTier: String? = null,
    val error: String? = null,
)

@HiltViewModel
class SubscriptionViewModel @Inject constructor(
    private val getSubscriptionUseCase: GetSubscriptionUseCase,
) : ViewModel() {

    private val _uiState = MutableStateFlow(SubscriptionUiState())
    val uiState: StateFlow<SubscriptionUiState> = _uiState

    init {
        loadSubscription()
    }

    fun loadSubscription() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
            getSubscriptionUseCase()
                .onSuccess { sub ->
                    _uiState.update { it.copy(isLoading = false, currentTier = sub.tier) }
                }
                .onFailure { e ->
                    _uiState.update { it.copy(isLoading = false, error = e.message) }
                }
        }
    }
}
