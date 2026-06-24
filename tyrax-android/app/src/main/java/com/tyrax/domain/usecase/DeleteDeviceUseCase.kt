package com.tyrax.domain.usecase

import com.tyrax.domain.repository.VpnRepository
import javax.inject.Inject

class DeleteDeviceUseCase @Inject constructor(
    private val vpnRepository: VpnRepository,
) {
    suspend operator fun invoke(deviceId: String): Result<Unit> =
        vpnRepository.deleteDevice(deviceId)
}
