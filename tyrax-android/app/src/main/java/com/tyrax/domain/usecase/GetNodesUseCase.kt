package com.tyrax.domain.usecase

import com.tyrax.domain.model.Node
import com.tyrax.domain.repository.VpnRepository
import javax.inject.Inject

class GetNodesUseCase @Inject constructor(
    private val vpnRepository: VpnRepository,
) {
    suspend operator fun invoke(): Result<List<Node>> = vpnRepository.getNodes()
}
