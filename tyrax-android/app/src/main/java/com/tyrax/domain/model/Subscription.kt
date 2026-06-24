package com.tyrax.domain.model

import kotlin.math.roundToInt

data class Subscription(
    val tier: String,
    val endsAt: String?,
    val devicesCount: Int,
    val devicesLimit: Int,
) {
    val isFree: Boolean get() = tier.uppercase() == "FREE"
}

data class UserDevice(
    val id: String,
    val name: String,
    val createdAt: String,
)

data class InviteRecord(
    val id: String,
    val inviteeId: String,
    val status: String,
)

// Pricing helpers — mirrors PAYMENTS.md exactly.
object Pricing {
    private val basePrices = mapOf("CORE" to 199.0, "SHADOW" to 349.0, "DOMINION" to 649.0)
    private val discounts  = mapOf(1 to 1.00, 3 to 0.90, 6 to 0.85, 12 to 0.80)

    fun calculate(tier: String, months: Int): Double {
        val base     = basePrices[tier] ?: return 0.0
        val discount = discounts[months] ?: 1.0
        return (base * months * discount).roundToInt().toDouble()
    }

    fun discountLabel(months: Int): String? = when (months) {
        3    -> "-10%"
        6    -> "-15%"
        12   -> "-20%"
        else -> null
    }
}
