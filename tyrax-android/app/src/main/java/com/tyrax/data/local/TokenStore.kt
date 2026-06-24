package com.tyrax.data.local

import kotlinx.coroutines.flow.Flow
import javax.inject.Inject
import javax.inject.Singleton

/**
 * High-level token store used by the auth / network layer.
 * Delegates to [TokenDataStore] so a single DataStore instance backs both the
 * splash-screen check and the auth / interceptor logic.
 */
@Singleton
class TokenStore @Inject constructor(
    private val tokenDataStore: TokenDataStore,
) {
    val token: Flow<String?> = tokenDataStore.token

    val isLoggedIn: Flow<Boolean> = tokenDataStore.hasToken

    val email: Flow<String?> = tokenDataStore.email

    val deviceName: Flow<String?> = tokenDataStore.deviceName

    suspend fun saveToken(token: String) = tokenDataStore.saveToken(token)

    suspend fun saveEmail(email: String) = tokenDataStore.saveEmail(email)

    suspend fun clearToken() = tokenDataStore.clearToken()

    suspend fun getOrCreateDeviceName(): String = tokenDataStore.getOrCreateDeviceName()
}
