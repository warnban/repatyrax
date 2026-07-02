using System.Text.Json;

namespace Tyrax.Tunnel;

/// <summary>
/// Reads facts out of a raw Xray config JSON (as produced by the backend
/// <c>/vpn/connect</c>) without rebuilding it. Used by the service to find the
/// node address it must pin through the physical gateway.
/// </summary>
public static class XrayConfigInspector
{
    /// <summary>
    /// Returns the VLESS outbound's <c>vnext[0].address</c> (the node host to
    /// exclude from the tunnel), or <c>null</c> if the config has no proxy outbound.
    /// </summary>
    public static string? GetProxyHost(string configJson)
    {
        try
        {
            using var doc = JsonDocument.Parse(configJson);
            if (!doc.RootElement.TryGetProperty("outbounds", out var outbounds) ||
                outbounds.ValueKind != JsonValueKind.Array)
                return null;

            // Prefer the outbound tagged "proxy"; fall back to the first vless one.
            JsonElement? proxy = null;
            foreach (var ob in outbounds.EnumerateArray())
            {
                var tag = ob.TryGetProperty("tag", out var t) ? t.GetString() : null;
                var protocol = ob.TryGetProperty("protocol", out var p) ? p.GetString() : null;
                if (tag == "proxy" || protocol == "vless")
                {
                    proxy = ob;
                    if (tag == "proxy") break;
                }
            }
            if (proxy is null) return null;

            if (proxy.Value.TryGetProperty("settings", out var settings) &&
                settings.TryGetProperty("vnext", out var vnext) &&
                vnext.ValueKind == JsonValueKind.Array &&
                vnext.GetArrayLength() > 0 &&
                vnext[0].TryGetProperty("address", out var address))
            {
                return address.GetString();
            }
        }
        catch (JsonException)
        {
            // malformed config — caller treats a null host as unrecoverable
        }
        return null;
    }
}
