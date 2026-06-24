package com.tyrax.domain.usecase

import com.tyrax.data.remote.CreatePaymentRequest
import com.tyrax.data.remote.PaymentResultDto
import com.tyrax.data.remote.TyraxApiService
import javax.inject.Inject

data class PaymentResult(
    val orderId: String,
    val paymentUrl: String,
    val amountRub: Double,
)

class CreatePaymentUseCase @Inject constructor(
    private val api: TyraxApiService,
) {
    suspend operator fun invoke(
        tier: String,
        paymentMethod: String,
        months: Int,
        email: String,
    ): Result<PaymentResult> = runCatching {
        val resp = api.createPayment(
            CreatePaymentRequest(
                tier          = tier,
                paymentMethod = paymentMethod,
                months        = months,
                email         = email,
                ip            = "",  // backend uses c.IP() when empty
            )
        )
        val data = resp.data ?: error(resp.message ?: "PAYMENT CREATION FAILED")
        PaymentResult(
            orderId    = data.orderId,
            paymentUrl = data.paymentUrl,
            amountRub  = data.amountRub,
        )
    }
}
