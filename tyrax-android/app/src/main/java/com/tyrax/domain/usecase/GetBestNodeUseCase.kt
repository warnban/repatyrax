package com.tyrax.domain.usecase

import com.tyrax.domain.model.Node
import com.tyrax.domain.repository.VpnRepository
import javax.inject.Inject

class GetBestNodeUseCase @Inject constructor(
    private val vpnRepository: VpnRepository,
) {
    suspend operator fun invoke(): Result<Node> = vpnRepository.getBestNode()
}
