using System.Text.Json;
using System.Text.Json.Nodes;

namespace Tyrax.Tunnel;

/// <summary>
/// Rewrites the backend SOCKS profile into a Windows-native full-tunnel profile:
/// Xray TUN inbound (WinTun), no FakeDNS, no sniffing, simplified DNS.
/// Outbounds and VLESS transport stay identical to the server response.
/// </summary>
public static class XrayWindowsConfigAdapter
{
    public const string TunAdapterName = "TYRAX";
    public const int TunMtu = 1400;
    public const string PrimaryDns = "1.1.1.1";

    /// <summary>
    /// Transforms <paramref name="backendConfigJson"/> (SOCKS inbound from
    /// <c>/vpn/connect</c>) into a config xray.exe runs with a native TUN device.
    /// </summary>
    public static string AdaptForNativeTun(string backendConfigJson)
    {
        var root = JsonNode.Parse(backendConfigJson)?.AsObject()
            ?? throw new ArgumentException("INVALID XRAY CONFIG", nameof(backendConfigJson));

        root.Remove("fakedns");

        root["dns"] = new JsonObject
        {
            ["queryStrategy"] = "UseIPv4",
            ["servers"] = new JsonArray
            {
                new JsonObject
                {
                    ["address"] = PrimaryDns,
                    ["port"] = 53,
                    ["proxyTag"] = "proxy",
                },
            },
        };

        root["inbounds"] = new JsonArray(CreateTunInbound());
        CleanOutbounds(root);
        CleanRouting(root);

        return root.ToJsonString(new JsonSerializerOptions { WriteIndented = true });
    }

    private static JsonObject CreateTunInbound() => new()
    {
        ["port"] = 0,
        ["protocol"] = "tun",
        ["tag"] = "tun-in",
        ["settings"] = new JsonObject
        {
            ["name"] = TunAdapterName,
            ["mtu"] = TunMtu,
        },
    };

    private static void CleanOutbounds(JsonObject root)
    {
        if (root["outbounds"] is not JsonArray outbounds) return;

        var kept = new JsonArray();
        foreach (var ob in outbounds)
        {
            if (ob is JsonObject o && o["tag"]?.GetValue<string>() == "dns-out")
                continue;
            kept.Add(ob?.DeepClone());
        }
        root["outbounds"] = kept;
    }

    private static void CleanRouting(JsonObject root)
    {
        if (root["routing"] is not JsonObject routing ||
            routing["rules"] is not JsonArray rules)
            return;

        var kept = new JsonArray();
        foreach (var rule in rules)
        {
            if (rule is not JsonObject r) continue;

            if (r["outboundTag"]?.GetValue<string>() == "dns-out")
                continue;

            if (r["ip"] is JsonArray ips &&
                ips.Any(i => i?.GetValue<string>()?.StartsWith("198.18.", StringComparison.Ordinal) == true))
                continue;

            kept.Add(r.DeepClone());
        }
        routing["rules"] = kept;
    }
}
