namespace Tyrax.Core.Models;

/// <summary>
/// Ready-to-run tunnel config from <c>POST /vpn/connect</c>. <see cref="Config"/>
/// is the raw engine config (Xray JSON for vless) the service runs verbatim.
/// </summary>
public sealed record ConnectConfig
{
    public required string Protocol { get; init; }
    public required string Config { get; init; }
}
