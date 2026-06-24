package com.tyrax.presentation.screens.nodes

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.tyrax.domain.model.Node
import com.tyrax.domain.usecase.GetNodesUseCase
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

sealed class NodesUiState {
    object Loading : NodesUiState()
    data class Success(val nodes: List<Node>) : NodesUiState()
    data class Error(val message: String) : NodesUiState()
}

@HiltViewModel
class NodesViewModel @Inject constructor(
    private val getNodesUseCase: GetNodesUseCase,
) : ViewModel() {

    private val _uiState = MutableStateFlow<NodesUiState>(NodesUiState.Loading)
    val uiState: StateFlow<NodesUiState> = _uiState

    init {
        loadNodes()
    }

    fun loadNodes() {
        viewModelScope.launch {
            _uiState.value = NodesUiState.Loading
            getNodesUseCase()
                .onSuccess { nodes -> _uiState.value = NodesUiState.Success(nodes) }
                .onFailure { error -> _uiState.value = NodesUiState.Error(error.message ?: "NODE UNAVAILABLE") }
        }
    }
}
