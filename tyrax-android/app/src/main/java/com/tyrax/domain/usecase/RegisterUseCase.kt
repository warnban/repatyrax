package com.tyrax.domain.usecase

import com.tyrax.domain.model.AuthData
import com.tyrax.domain.repository.AuthRepository
import javax.inject.Inject

class RegisterUseCase @Inject constructor(
    private val authRepository: AuthRepository,
) {
    suspend operator fun invoke(email: String, password: String): Result<AuthData> =
        authRepository.register(email, password)
}
