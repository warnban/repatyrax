using System.Text.Json.Serialization;

namespace Tyrax.Data.Remote;

// ── Auth ─────────────────────────────────────────────────────────────────────

public sealed record RegisterRequest(
    [property: JsonPropertyName("email")] string Email,
    [property: JsonPropertyName("password")] string Password);

public sealed record LoginRequest(
    [property: JsonPropertyName("email")] string Email,
    [property: JsonPropertyName("password")] string Password);

public sealed record VerifyRequest(
    [property: JsonPropertyName("email")] string Email,
    [property: JsonPropertyName("code")] string Code);

public sealed record ResendRequest(
    [property: JsonPropertyName("email")] string Email);

public sealed class AuthDataDto
{
    [JsonPropertyName("token")] public string Token { get; set; } = "";
    [JsonPropertyName("user_id")] public string? UserId { get; set; }
    [JsonPropertyName("email")] public string? Email { get; set; }
    [JsonPropertyName("email_verified")] public bool EmailVerified { get; set; }
    [JsonPropertyName("verification_required")] public bool VerificationRequired { get; set; }
}

public sealed class TelegramInitDataDto
{
    [JsonPropertyName("bot_url")] public string BotUrl { get; set; } = "";
    [JsonPropertyName("token")] public string Token { get; set; } = "";
}

public sealed class ProfileDataDto
{
    [JsonPropertyName("user_id")] public string? UserId { get; set; }
    [JsonPropertyName("email")] public string? Email { get; set; }
    [JsonPropertyName("tier")] public string? Tier { get; set; }
    [JsonPropertyName("telegram_linked")] public bool TelegramLinked { get; set; }
}

// ── Nodes ─────────────────────────────────────────────────────────────────────

public sealed class NodeDto
{
    [JsonPropertyName("id")] public string Id { get; set; } = "";
    [JsonPropertyName("codename")] public string Codename { get; set; } = "";
    [JsonPropertyName("country")] public string Country { get; set; } = "";
    [JsonPropertyName("status")] public string Status { get; set; } = "";
    [JsonPropertyName("ping_ms")] public int PingMs { get; set; }
    [JsonPropertyName("load")] public int Load { get; set; } = -1;
    [JsonPropertyName("reality_sni")] public string? RealitySni { get; set; }
}

// ── VPN ───────────────────────────────────────────────────────────────────────

public sealed record AddDeviceRequest(
    [property: JsonPropertyName("name")] string Name);

public sealed record VpnConnectRequest(
    [property: JsonPropertyName("name")] string Name,
    [property: JsonPropertyName("codename")] string Codename);

public sealed class VpnConfigDto
{
    [JsonPropertyName("protocol")] public string Protocol { get; set; } = "";
    [JsonPropertyName("config")] public string Config { get; set; } = "";
}

public sealed class SplitDomainsDto
{
    [JsonPropertyName("domains")] public List<string> Domains { get; set; } = new();
    [JsonPropertyName("updated_at")] public string? UpdatedAt { get; set; }
}

public sealed class UserDeviceDto
{
    [JsonPropertyName("id")] public string Id { get; set; } = "";
    [JsonPropertyName("name")] public string Name { get; set; } = "";
    [JsonPropertyName("created_at")] public string? CreatedAt { get; set; }
}

// ── Subscription / invites ──────────────────────────────────────────────────

public sealed class SubscriptionDto
{
    [JsonPropertyName("tier")] public string Tier { get; set; } = "";
    [JsonPropertyName("ends_at")] public string? EndsAt { get; set; }
    [JsonPropertyName("devices_count")] public int DevicesCount { get; set; }
    [JsonPropertyName("devices_limit")] public int DevicesLimit { get; set; }
    [JsonPropertyName("traffic_used_bytes")] public long TrafficUsedBytes { get; set; }
    [JsonPropertyName("traffic_limit_bytes")] public long TrafficLimitBytes { get; set; }
    [JsonPropertyName("blocked_until")] public string? BlockedUntil { get; set; }
    [JsonPropertyName("unlimited")] public bool Unlimited { get; set; }
}

public sealed class InviteDto
{
    [JsonPropertyName("id")] public string Id { get; set; } = "";
    [JsonPropertyName("invitee_id")] public string InviteeId { get; set; } = "";
    [JsonPropertyName("status")] public string Status { get; set; } = "";
}

public sealed record SendInviteRequest(
    [property: JsonPropertyName("account_id")] string AccountId);

// ── Payments ────────────────────────────────────────────────────────────────

public sealed record CreatePaymentRequest(
    [property: JsonPropertyName("tier")] string Tier,
    [property: JsonPropertyName("payment_method")] string PaymentMethod,
    [property: JsonPropertyName("months")] int Months,
    [property: JsonPropertyName("email")] string Email,
    [property: JsonPropertyName("ip")] string Ip);

public sealed class PaymentResultDto
{
    [JsonPropertyName("order_id")] public string OrderId { get; set; } = "";
    [JsonPropertyName("payment_url")] public string PaymentUrl { get; set; } = "";
    [JsonPropertyName("amount_rub")] public double AmountRub { get; set; }
}

public sealed class PaymentStatusDto
{
    [JsonPropertyName("order_status")] public string OrderStatus { get; set; } = "";
    [JsonPropertyName("tier")] public string Tier { get; set; } = "";
}
