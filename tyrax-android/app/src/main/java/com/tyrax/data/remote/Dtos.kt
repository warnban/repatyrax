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

/** Body for POST /vpn/connect — server requires device name + target node codename. */
data class VpnConnectRequest(
    val name: String,
    val codename: String,
)

data class DeviceConfigDto(
    @SerializedName("device_id")      val deviceId: String,
    val protocol: String? = null, // "wireguard" | "vless"
    @SerializedName("wireguard_conf") val wireguardConf: String? = null,
    @SerializedName("vless_conf")     val vlessConf: String? = null,
    // Structured VLESS + Reality params (present when protocol == "vless").
    val uuid: String? = null,
    @SerializedName("node_host")           val nodeHost: String? = null,
    @SerializedName("node_port")           val nodePort: Int? = null,
    @SerializedName("reality_public_key")  val realityPublicKey: String? = null,
    @SerializedName("reality_sni")         val realitySni: String? = null,
    @SerializedName("reality_short_id")    val realityShortId: String? = null,
    // Transport / anti-DPI params (RU 2026); present when protocol == "vless".
    val security: String? = null,
    val network: String? = null,
    val flow: String? = null,
    @SerializedName("xhttp_path")          val xhttpPath: String? = null,
    @SerializedName("xhttp_mode")          val xhttpMode: String? = null,
    @SerializedName("x_padding_bytes")     val xPaddingBytes: String? = null,
    val fingerprint: String? = null,
    val nodes: List<NodeDto> = emptyList(),
)

data class VpnConfigDto(
    val protocol: String,
    /** Raw WireGuard conf or Xray JSON string — field name is always `config`. */
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
    // Traffic metering. limit == -1 means unlimited (paid tiers). blockedUntil is
    // set only while a FREE identity is locked out after exhausting its quota.
    @SerializedName("traffic_used_bytes")  val trafficUsedBytes: Long = 0,
    @SerializedName("traffic_limit_bytes") val trafficLimitBytes: Long = 0,
    @SerializedName("blocked_until")       val blockedUntil: String? = null,
    val unlimited: Boolean = false,
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
