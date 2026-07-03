using Refit;

namespace Tyrax.Data.Remote;

/// <summary>
/// Refit surface for the TYRAX backend (<c>https://api.tyrax.tech/api/v1</c>).
/// Mirrors the Android <c>TyraxApiService</c> one-to-one. The Bearer header is
/// attached by <see cref="AuthHeaderHandler"/>, not per-method.
/// </summary>
public interface ITyraxApi
{
    // ── Auth ─────────────────────────────────────────────────────────────────
    [Post("/auth/register")]
    Task<ApiEnvelope<AuthDataDto>> RegisterAsync([Body] RegisterRequest req, CancellationToken ct = default);

    [Post("/auth/login")]
    Task<ApiEnvelope<AuthDataDto>> LoginAsync([Body] LoginRequest req, CancellationToken ct = default);

    [Post("/auth/verify")]
    Task<ApiEnvelope<AuthDataDto>> VerifyEmailAsync([Body] VerifyRequest req, CancellationToken ct = default);

    [Post("/auth/resend-verification")]
    Task<ApiEnvelope<ResendDataDto>> ResendVerificationAsync([Body] ResendRequest req, CancellationToken ct = default);

    [Get("/auth/telegram-init")]
    Task<ApiEnvelope<TelegramInitDataDto>> TelegramInitAsync(CancellationToken ct = default);

    [Get("/auth/telegram-status")]
    Task<ApiEnvelope<AuthDataDto>> TelegramStatusAsync([Query] string token, CancellationToken ct = default);

    [Get("/auth/profile")]
    Task<ApiEnvelope<ProfileDataDto>> GetProfileAsync(CancellationToken ct = default);

    // ── Nodes ─────────────────────────────────────────────────────────────────
    [Get("/nodes")]
    Task<ApiEnvelope<List<NodeDto>>> GetNodesAsync(CancellationToken ct = default);

    // ── VPN ───────────────────────────────────────────────────────────────────
    [Post("/vpn/device")]
    Task<ApiEnvelope<object>> AddDeviceAsync([Body] AddDeviceRequest req, CancellationToken ct = default);

    [Post("/vpn/connect")]
    Task<ApiEnvelope<VpnConfigDto>> ConnectAsync([Body] VpnConnectRequest req, CancellationToken ct = default);

    [Delete("/vpn/device/{id}")]
    Task<ApiEnvelope<object>> DeleteDeviceAsync(string id, CancellationToken ct = default);

    [Get("/vpn/devices")]
    Task<ApiEnvelope<List<UserDeviceDto>>> GetDevicesAsync(CancellationToken ct = default);

    [Get("/vpn/split-domains")]
    Task<ApiEnvelope<SplitDomainsDto>> GetSplitDomainsAsync(CancellationToken ct = default);

    // ── Subscription / invites ─────────────────────────────────────────────────
    [Get("/subscription")]
    Task<ApiEnvelope<SubscriptionDto>> GetSubscriptionAsync(CancellationToken ct = default);

    [Get("/subscription/invites")]
    Task<ApiEnvelope<List<InviteDto>>> GetInvitesAsync(CancellationToken ct = default);

    [Post("/subscription/invite")]
    Task<ApiEnvelope<object>> SendInviteAsync([Body] SendInviteRequest req, CancellationToken ct = default);

    [Delete("/subscription/invite/{accountId}")]
    Task<ApiEnvelope<object>> RemoveInviteAsync(string accountId, CancellationToken ct = default);

    // ── Payments ────────────────────────────────────────────────────────────────
    [Post("/payment/create")]
    Task<ApiEnvelope<PaymentResultDto>> CreatePaymentAsync([Body] CreatePaymentRequest req, CancellationToken ct = default);

    [Get("/payment/status/{orderId}")]
    Task<ApiEnvelope<PaymentStatusDto>> GetPaymentStatusAsync(string orderId, CancellationToken ct = default);
}
