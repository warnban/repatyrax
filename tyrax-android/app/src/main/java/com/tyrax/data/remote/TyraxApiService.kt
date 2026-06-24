package com.tyrax.data.remote

import retrofit2.http.*

// Production base URL.
// For emulator local dev use: http://10.0.2.2:8080/api/v1/
const val BASE_URL = "https://api.tyrax.tech/api/v1/"

interface TyraxApiService {

    // ── Auth ─────────────────────────────────────────────────────────────────

    @POST("auth/register")
    suspend fun register(@Body req: RegisterRequest): ApiResponse<AuthDataDto>

    @POST("auth/login")
    suspend fun login(@Body req: LoginRequest): ApiResponse<AuthDataDto>

    @GET("auth/telegram-init")
    suspend fun telegramInit(): ApiResponse<TelegramInitDataDto>

    // Called by the client after the user completes the Telegram bot flow.
    // The backend must implement GET /auth/telegram-status?token=INIT_TOKEN
    // and return AuthDataDto once the bot has confirmed the user.
    @GET("auth/telegram-status")
    suspend fun getTelegramStatus(@Query("token") initToken: String): ApiResponse<AuthDataDto>

    @GET("auth/profile")
    suspend fun getProfile(): ApiResponse<ProfileDataDto>

    // ── Nodes ─────────────────────────────────────────────────────────────────

    @GET("nodes")
    suspend fun getNodes(): ApiResponse<List<NodeDto>>

    // ── VPN ───────────────────────────────────────────────────────────────────

    @GET("vpn/config")
    suspend fun getVpnConfig(@Query("device_public_key") pubKey: String): ApiResponse<VpnConfigDto>

    @POST("vpn/device")
    suspend fun addDevice(@Body req: AddDeviceRequest): ApiResponse<DeviceConfigDto>

    @DELETE("vpn/device/{id}")
    suspend fun deleteDevice(@Path("id") deviceId: String): ApiResponse<Unit>

    @GET("vpn/devices")
    suspend fun getDevices(): ApiResponse<List<UserDeviceDto>>

    @GET("vpn/split-domains")
    suspend fun getSplitDomains(): ApiResponse<SplitDomainsDto>

    // ── Subscription ──────────────────────────────────────────────────────────

    @GET("subscription")
    suspend fun getSubscription(): ApiResponse<SubscriptionDto>

    // ── Payments ──────────────────────────────────────────────────────────────

    // ── Invites (DOMINION) ────────────────────────────────────────────────────

    @GET("subscription/invites")
    suspend fun getInvites(): ApiResponse<List<InviteDto>>

    @POST("subscription/invite")
    suspend fun sendInvite(@Body req: SendInviteRequest): ApiResponse<Unit>

    @DELETE("subscription/invite/{accountID}")
    suspend fun removeInvite(@Path("accountID") accountId: String): ApiResponse<Unit>

    @POST("payment/create")
    suspend fun createPayment(@Body req: CreatePaymentRequest): ApiResponse<PaymentResultDto>

    @GET("payment/status/{orderId}")
    suspend fun getPaymentStatus(@Path("orderId") orderId: String): ApiResponse<PaymentStatusDto>
}
