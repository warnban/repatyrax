package com.tyrax.presentation.screens.payment

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.tyrax.data.local.TokenStore
import com.tyrax.domain.usecase.CreatePaymentUseCase
import com.tyrax.domain.usecase.GetPaymentStatusUseCase
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
import kotlin.math.roundToInt

data class PaymentUiState(
    val tier: String = "CORE",
    val selectedMonths: Int = 1,
    val selectedMethod: String = "SBP",
    val total: Int = 0,
    val monthly: Int = 0,
    val saving: Int = 0,
    val isLoading: Boolean = false,
    val error: String? = null,
    val paymentSuccess: Boolean = false,
)

sealed class PaymentEvent {
    data class OpenUrl(val url: String) : PaymentEvent()
}

@HiltViewModel
class PaymentViewModel @Inject constructor(
    savedStateHandle: SavedStateHandle,
    private val createPaymentUseCase: CreatePaymentUseCase,
    private val getPaymentStatusUseCase: GetPaymentStatusUseCase,
    private val tokenStore: TokenStore,
) : ViewModel() {

    private val tier: String = savedStateHandle.get<String>("tier")?.uppercase() ?: "CORE"

    private val _uiState = MutableStateFlow(PaymentUiState(tier = tier))
    val uiState: StateFlow<PaymentUiState> = _uiState

    private val _events = Channel<PaymentEvent>(Channel.BUFFERED)
    val events = _events.receiveAsFlow()

    init {
        recompute(1)
    }

    fun selectMonths(months: Int) = recompute(months)

    fun selectMethod(method: String) {
        _uiState.update { it.copy(selectedMethod = method) }
    }

    private fun recompute(months: Int) {
        val base = BASE_PRICES[tier] ?: 0
        val discount = DISCOUNTS[months] ?: 1.0
        val total = (base * months * discount).roundToInt()
        val monthly = (total.toDouble() / months).roundToInt()
        val saving = base * months - total
        _uiState.update {
            it.copy(
                selectedMonths = months,
                total          = total,
                monthly        = monthly,
                saving         = saving,
            )
        }
    }

    fun pay() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
            val email = tokenStore.email.firstOrNull().orEmpty()
            val state = _uiState.value
            createPaymentUseCase(state.tier, state.selectedMethod, state.selectedMonths, email)
                .onSuccess { result ->
                    _events.send(PaymentEvent.OpenUrl(result.paymentUrl))
                    pollStatus(result.orderId)
                }
                .onFailure { e ->
                    _uiState.update { it.copy(isLoading = false, error = e.message) }
                }
        }
    }

    // Poll every 3s for up to 5 minutes (100 attempts) until the order is PAID.
    private fun pollStatus(orderId: String) {
        viewModelScope.launch {
            repeat(100) {
                delay(3_000)
                val status = getPaymentStatusUseCase(orderId).getOrNull()
                if (status == "PAID") {
                    _uiState.update { it.copy(isLoading = false, paymentSuccess = true) }
                    return@launch
                }
            }
            _uiState.update { it.copy(isLoading = false) }
        }
    }

    private companion object {
        val BASE_PRICES = mapOf("CORE" to 199, "SHADOW" to 349, "DOMINION" to 649)
        val DISCOUNTS = mapOf(1 to 1.0, 3 to 0.90, 6 to 0.85, 12 to 0.80)
    }
}
