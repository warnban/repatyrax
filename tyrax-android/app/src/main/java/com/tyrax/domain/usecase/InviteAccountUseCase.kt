package com.tyrax.domain.usecase

import com.tyrax.domain.repository.VpnRepository
import javax.inject.Inject

class InviteAccountUseCase @Inject constructor(
    private val vpnRepository: VpnRepository,
) {
    suspend fun send(accountId: String): Result<Unit>   = vpnRepository.sendInvite(accountId)
    suspend fun remove(accountId: String): Result<Unit> = vpnRepository.removeInvite(accountId)
}
