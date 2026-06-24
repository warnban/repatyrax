package com.tyrax.domain.usecase

import com.tyrax.data.remote.TyraxApiService
import javax.inject.Inject

// Returns the order status string ("NEW" / "PAID" / "CANCELLED" / …).
class GetPaymentStatusUseCase @Inject constructor(
    private val api: TyraxApiService,
) {
    suspend operator fun invoke(orderId: String): Result<String> = runCatching {
        val resp = api.getPaymentStatus(orderId)
        val data = resp.data ?: error(resp.message ?: "STATUS UNAVAILABLE")
        data.orderStatus
    }
}
