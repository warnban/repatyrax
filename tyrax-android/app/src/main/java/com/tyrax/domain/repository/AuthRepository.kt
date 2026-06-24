package com.tyrax.domain.repository

import com.tyrax.domain.model.AuthData
import kotlinx.coroutines.flow.Flow

interface AuthRepository {
    suspend fun login(email: String, password: String): Result<AuthData>
    suspend fun register(email: String, password: String): Result<AuthData>
    suspend fun getTelegramInitUrl(): Result<TelegramInitResult>
    suspend fun pollTelegramStatus(initToken: String): Result<AuthData>
    suspend fun getProfile(): Result<UserProfile>
    suspend fun saveToken(token: String)
    suspend fun logout()
    val isLoggedIn: Flow<Boolean>
}

data class TelegramInitResult(
    val botUrl: String,
    val initToken: String,
)

data class UserProfile(
    val userId: String?,
    val email: String?,
    val tier: String?,
    val telegramLinked: Boolean,
)
