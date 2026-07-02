namespace Tyrax.Core.Models;

/// <summary>Current plan + device/traffic metering, from <c>/subscription</c>.</summary>
public sealed record Subscription
{
    public required string Tier { get; init; }
    public string? EndsAt { get; init; }
    public int DevicesCount { get; init; }
    public int DevicesLimit { get; init; }

    /// <summary>Traffic used / cap in bytes. <c>Unlimited</c> is set on paid tiers.</summary>
    public long TrafficUsedBytes { get; init; }
    public long TrafficLimitBytes { get; init; }
    public bool Unlimited { get; init; }

    /// <summary>Set only while a FREE identity is locked out after exhausting quota.</summary>
    public string? BlockedUntil { get; init; }
}

/// <summary>A registered device in the account's "My Devices" list.</summary>
public sealed record UserDevice
{
    public required string Id { get; init; }
    public required string Name { get; init; }
    public string? CreatedAt { get; init; }
}

/// <summary>A DOMINION invite relationship.</summary>
public sealed record Invite
{
    public required string Id { get; init; }
    public required string InviteeId { get; init; }
    public required string Status { get; init; }
}
