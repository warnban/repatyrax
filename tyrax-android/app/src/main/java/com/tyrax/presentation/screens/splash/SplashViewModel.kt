package com.tyrax.presentation.screens.splash

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.tyrax.data.local.TokenDataStore
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.stateIn
import javax.inject.Inject

@HiltViewModel
class SplashViewModel @Inject constructor(
    tokenDataStore: TokenDataStore,
) : ViewModel() {

    // null = still loading, true = has token → go to Main, false = no token → Onboarding
    val hasToken: StateFlow<Boolean?> = tokenDataStore.hasToken
        .stateIn(
            scope         = viewModelScope,
            started       = SharingStarted.WhileSubscribed(5_000),
            initialValue  = null,
        )
}
