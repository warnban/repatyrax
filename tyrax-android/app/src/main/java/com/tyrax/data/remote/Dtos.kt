package com.tyrax.data.remote

import com.google.gson.annotations.SerializedName

// ── Auth ─────────────────────────────────────────────────────────────────────

data class RegisterRequest(
    val email: String,
    val password: String,
)

data class LoginRequest(
    val email: String,
    val password: String,
)

data class AuthDataDto(
    val token: String,
    @SerializedName("user_id") val userId: String? = null,
)

data class TelegramInitDataDto(
    @SerializedName("bot_url") val botUrl: String,
    val token: String,
)

data class ProfileDataDto(
    @SerializedName("user_id") val userId: String? = null,
    val email: String? = null,
    val tier: String? = null,
    @SerializedName("telegram_linked") val telegramLinked: Boolean = false,
)

// ── Nodes ─────────────────────────────────────────────────────────────────────

data class NodeDto(
    val id: String,
    val codename: String,
    val country: String,
    val status: String,
    @SerializedName("ping_ms") val pingMs: Int,
    @SerializedName("reality_sni") val realitySni: String? = null,
)

// ── VPN ───────────────────────────────────────────────────────────────────────

data class AddDeviceRequest(
    val name: String,
)

data class DeviceConfigDto(
    @SerializedName("device_id")      val deviceId: String,
    @SerializedName("wireguard_conf") val wireguardConf: String? = null,
    @SerializedName("vless_conf")     val vlessConf: String? = null,
    val nodes: List<NodeDto> = emptyList(),
)

data class VpnConfigDto(
    val protocol: String,
    val config: String,
)

data class SplitDomainsDto(
    val domains: List<String> = emptyList(),
    @SerializedName("updated_at") val updatedAt: String? = null,
)

// ── Subscription ──────────────────────────────────────────────────────────────

data class SubscriptionDto(
    val tier: String,
    @SerializedName("ends_at")       val endsAt: String? = null,
    @SerializedName("devices_count") val devicesCount: Int,
    @SerializedName("devices_limit") val devicesLimit: Int,
)

// ── Payments ──────────────────────────────────────────────────────────────────

data class CreatePaymentRequest(
    val tier: String,
    @SerializedName("payment_method") val paymentMethod: String,
    val months: Int,
    val email: String,
    val ip: String,
)

data class PaymentResultDto(
    @SerializedName("order_id")    val orderId: String,
    @SerializedName("payment_url") val paymentUrl: String,
    @SerializedName("amount_rub")  val amountRub: Double,
)

data class PaymentStatusDto(
    @SerializedName("order_status") val orderStatus: String,
    val tier: String,
)

// ── Devices ───────────────────────────────────────────────────────────────────

data class UserDeviceDto(
    val id: String,
    val name: String,
    @SerializedName("created_at") val createdAt: String,
)

// ── Invites ───────────────────────────────────────────────────────────────────

data class InviteDto(
    val id: String,
    @SerializedName("invitee_id") val inviteeId: String,
    val status: String,
)

data class SendInviteRequest(
    @SerializedName("account_id") val accountId: String,
)
