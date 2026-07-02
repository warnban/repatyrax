namespace Tyrax.Core.Models;

/// <summary>
/// A TYRAX exit NODE as returned by the backend <c>GET /nodes</c> endpoint.
/// Mirrors the Android <c>NodeDto</c>. No flags — codenames only (NL-01, DE-02…).
/// </summary>
public sealed record Node
{
    public required string Id { get; init; }
    public required string Codename { get; init; }
    public required string Country { get; init; }

    /// <summary>OPEN · MONITORED · HEAVILY RESTRICTED.</summary>
    public required string Status { get; init; }

    public int PingMs { get; init; }

    /// <summary>Live online-client count for balancing, or -1 when unknown.</summary>
    public int Load { get; init; } = -1;

    public string? RealitySni { get; init; }
}
