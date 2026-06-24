package com.tyrax.domain.usecase

import com.tyrax.domain.model.DeviceConfig
import com.tyrax.domain.repository.VpnRepository
import javax.inject.Inject

class AddDeviceUseCase @Inject constructor(
    private val vpnRepository: VpnRepository,
) {
    suspend operator fun invoke(name: String): Result<DeviceConfig> =
        vpnRepository.addDevice(name)
}
