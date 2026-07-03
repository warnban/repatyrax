package com.tyrax.domain.model

data class AuthData(
    val token: String,
    val userId: String?,
    val email: String? = null,
    val emailVerified: Boolean = false,
    // True on the register response when the backend requires email confirmation
    // before issuing a session (no token yet — route to the verify screen).
    val verificationRequired: Boolean = false,
)
