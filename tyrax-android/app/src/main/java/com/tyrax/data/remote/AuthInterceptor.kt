package com.tyrax.data.remote

import com.tyrax.data.local.TokenStore
import kotlinx.coroutines.flow.firstOrNull
import kotlinx.coroutines.runBlocking
import okhttp3.Interceptor
import okhttp3.Response

// Endpoints that must NOT carry a JWT — adding one here would cause an auth loop.
private val NO_AUTH_PATHS = setOf("auth/register", "auth/login")

class AuthInterceptor(private val tokenStore: TokenStore) : Interceptor {

    override fun intercept(chain: Interceptor.Chain): Response {
        val request = chain.request()
        val path = request.url.encodedPath

        val needsAuth = NO_AUTH_PATHS.none { path.endsWith(it) }
        if (!needsAuth) return chain.proceed(request)

        // runBlocking is acceptable inside OkHttp interceptors — they already run on IO threads.
        val token: String? = runBlocking { tokenStore.token.firstOrNull() }

        val authenticatedRequest = if (!token.isNullOrBlank()) {
            request.newBuilder()
                .addHeader("Authorization", "Bearer $token")
                .build()
        } else {
            request
        }

        return chain.proceed(authenticatedRequest)
    }
}
