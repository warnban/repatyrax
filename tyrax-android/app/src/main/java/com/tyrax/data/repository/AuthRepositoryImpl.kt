package com.tyrax.data.repository

import com.tyrax.data.local.TokenStore
import com.tyrax.data.remote.LoginRequest
import com.tyrax.data.remote.RegisterRequest
import com.tyrax.data.remote.ResendRequest
import com.tyrax.data.remote.TyraxApiService
import com.tyrax.data.remote.VerifyRequest
import com.tyrax.domain.model.AuthData
import com.tyrax.domain.repository.AuthRepository
import com.tyrax.domain.repository.TelegramInitResult
import com.tyrax.domain.repository.UserProfile
import kotlinx.coroutines.flow.Flow
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class AuthRepositoryImpl @Inject constructor(
    private val api: TyraxApiService,
    private val tokenStore: TokenStore,
) : AuthRepository {

    override suspend fun login(email: String, password: String): Result<AuthData> = runCatching {
        val resp = api.login(LoginRequest(email, password))
        val data = resp.data?.toAuthData() ?: error(resp.message ?: "INVALID CREDENTIALS")
        tokenStore.saveEmail(email)
        data
    }.mapApiError()

    override suspend fun register(email: String, password: String): Result<AuthData> = runCatching {
        val resp = api.register(RegisterRequest(email, password))
        val data = resp.data?.toAuthData() ?: error(resp.message ?: "REGISTRATION FAILED")
        tokenStore.saveEmail(email)
        data
    }.mapApiError()

    override suspend fun verifyEmail(email: String, code: String): Result<AuthData> = runCatching {
        val resp = api.verifyEmail(VerifyRequest(email, code))
        val data = resp.data?.toAuthData() ?: error(resp.message ?: "INVALID OR EXPIRED CODE")
        tokenStore.saveEmail(email)
        data
    }.mapApiError()

    override suspend fun resendVerification(email: String): Result<Unit> = runCatching {
        api.resendVerification(ResendRequest(email))
        Unit
    }.mapApiError()

    override suspend fun getTelegramInitUrl(): Result<TelegramInitResult> = runCatching {
        val resp = api.telegramInit()
        val data = resp.data ?: error("SYSTEM ERROR. NODE OFFLINE.")
        TelegramInitResult(botUrl = data.botUrl, initToken = data.token)
    }.mapApiError()

    override suspend fun pollTelegramStatus(initToken: String): Result<AuthData> = runCatching {
        val resp = api.getTelegramStatus(initToken)
        resp.data?.toAuthData() ?: error("IDENTITY NOT FOUND")
    }.mapApiError()

    override suspend fun getProfile(): Result<UserProfile> = runCatching {
        val resp = api.getProfile()
        val data = resp.data ?: error(resp.message ?: "IDENTITY NOT FOUND")
        UserProfile(
            userId         = data.userId,
            email          = data.email,
            tier           = data.tier,
            telegramLinked = data.telegramLinked,
        )
    }.mapApiError()

    override suspend fun saveToken(token: String) = tokenStore.saveToken(token)

    override suspend fun logout() = tokenStore.clearToken()

    override val isLoggedIn: Flow<Boolean> = tokenStore.isLoggedIn
}

private fun com.tyrax.data.remote.AuthDataDto.toAuthData() = AuthData(
    token                = token ?: "",
    userId               = userId,
    email                = email,
    emailVerified        = emailVerified,
    verificationRequired = verificationRequired,
)

// Map network / HTTP exceptions to TYRAX-branded messages.
private fun <T> Result<T>.mapApiError(): Result<T> = onFailure { e ->
    val message = when {
        e.message?.contains("401") == true -> "INVALID CREDENTIALS"
        e.message?.contains("403") == true -> "EMAIL NOT CONFIRMED"
        e.message?.contains("409") == true -> "IDENTITY ALREADY EXISTS"
        e.message?.contains("Unable to resolve") == true ||
        e.message?.contains("failed to connect") == true -> "CONNECTION FAILED. RETRY."
        else -> e.message ?: "SYSTEM ERROR. NODE OFFLINE."
    }
    return Result.failure(Exception(message))
}
