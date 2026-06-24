package com.tyrax.domain.usecase

import com.tyrax.domain.model.Subscription
import com.tyrax.domain.repository.VpnRepository
import javax.inject.Inject

class GetSubscriptionUseCase @Inject constructor(
    private val vpnRepository: VpnRepository,
) {
    suspend operator fun invoke(): Result<Subscription> = vpnRepository.getSubscription()
}
