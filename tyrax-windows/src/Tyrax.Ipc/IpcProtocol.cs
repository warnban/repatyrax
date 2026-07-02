using System.Text.Json;
using System.Text.Json.Serialization;
using Tyrax.Core.Models;

namespace Tyrax.Ipc;

/// <summary>
/// Named-pipe contract between the unprivileged WPF UI and the privileged
/// <c>TyraxService</c>. One newline-delimited JSON object per message. The UI
/// sends <see cref="IpcCommand"/>; the service pushes <see cref="IpcStatus"/>.
/// </summary>
public static class IpcProtocol
{
    /// <summary>
    /// Local named pipe. Prefixed with a fixed GUID-ish token so a rogue process
    /// squatting a common name cannot impersonate the service.
    /// </summary>
    public const string PipeName = "TYRAX.Protocol.Bridge.v1";

    public static readonly JsonSerializerOptions Json = new()
    {
        PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull,
        Converters = { new JsonStringEnumConverter() },
    };
}

public enum IpcCommandKind
{
    /// <summary>Bring the PROTOCOL up on <see cref="IpcCommand.Params"/> / <see cref="IpcCommand.Codename"/>.</summary>
    Connect = 0,

    /// <summary>Tear the PROTOCOL down.</summary>
    Disconnect = 1,

    /// <summary>Request an immediate <see cref="IpcStatus"/> push.</summary>
    Query = 2,
}

/// <summary>Command sent UI → service.</summary>
public sealed record IpcCommand
{
    public required IpcCommandKind Kind { get; init; }

    /// <summary>Target node codename (for <see cref="IpcCommandKind.Connect"/>).</summary>
    public string? Codename { get; init; }

    /// <summary>Protocol from <c>/vpn/connect</c>: <c>"vless"</c> (or <c>"wireguard"</c>).</summary>
    public string? Protocol { get; init; }

    /// <summary>
    /// Ready-to-run engine config as returned by the backend <c>/vpn/connect</c>
    /// (raw Xray JSON for vless). Used verbatim so client and server stay in
    /// lockstep — the service does not rebuild it.
    /// </summary>
    public string? ConfigJson { get; init; }

    /// <summary>Local SOCKS5 inbound port the backend config listens on (10808).</summary>
    public int SocksPort { get; init; } = 10808;

    /// <summary>RU split-tunnel domains that must exit via the direct outbound (Phase 6).</summary>
    public IReadOnlyList<string>? SplitDomains { get; init; }
}

/// <summary>Status pushed service → UI.</summary>
public sealed record IpcStatus
{
    public required TunnelState State { get; init; }
    public string? Codename { get; init; }

    /// <summary>Bytes uploaded / downloaded since the tunnel came up.</summary>
    public long TxBytes { get; init; }
    public long RxBytes { get; init; }

    /// <summary>On-brand message, e.g. "NODE UNAVAILABLE" when <see cref="State"/> is Error.</summary>
    public string? Message { get; init; }
}
