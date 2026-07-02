namespace Tyrax.Core.Models;

/// <summary>
/// Structured VLESS + Reality/XHTTP connection parameters, as returned by the
/// backend when <c>protocol == "vless"</c>. Direct port of the Android
/// <c>VlessConfig</c> data class so client and server stay in lockstep.
///
/// <para><see cref="Flow"/> is empty for the XHTTP profile and
/// <c>"xtls-rprx-vision"</c> for the stream-one Vision profile. <see cref="Network"/>
/// selects the Xray transport: <c>"xhttp"</c> (default, anti-DPI) or <c>"tcp"</c>
/// (legacy). XHTTP fields are ignored when <see cref="Network"/> is <c>"tcp"</c>.</para>
/// </summary>
public sealed record VlessParams
{
    public required string NodeHost { get; init; }
    public required int NodePort { get; init; }
    public required string UserUuid { get; init; }

    public string RealityPublicKey { get; init; } = "";
    public string RealitySni { get; init; } = "";
    public string RealityShortId { get; init; } = "";

    public string Flow { get; init; } = "";

    /// <summary>Stream security: <c>"reality"</c> (direct) or <c>"tls"</c> (CDN-fronted).</summary>
    public string Security { get; init; } = "reality";

    public string Network { get; init; } = "xhttp";
    public string XhttpPath { get; init; } = "/api/v1/data";
    public string XhttpMode { get; init; } = "auto";
    public string XPaddingBytes { get; init; } = "100-1000";
    public string Fingerprint { get; init; } = "chrome";

    /// <summary>Local SOCKS5 inbound the tun2socks bridge proxies through.</summary>
    public int SocksPort { get; init; } = 10808;
}
