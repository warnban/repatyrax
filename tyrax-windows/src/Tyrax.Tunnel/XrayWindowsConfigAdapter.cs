using System.Text.Json;
using System.Text.Json.Nodes;

namespace Tyrax.Tunnel;

/// <summary>
/// Rewrites the backend SOCKS profile into a Windows-native full-tunnel profile:
/// Xray TUN inbound (WinTun), DoH DNS, sniffing, xudp + XHTTP mux tuning, and
/// routing aligned with the Android <c>XrayConfigPatcher</c>.
/// </summary>
public static class XrayWindowsConfigAdapter
{
    public const string TunAdapterName = "TYRAX";
    public const int TunMtu = 1400;

    /// <summary>Must match <c>TunnelConstants.TunGateway</c> on the service side.</summary>
    public const string TunGateway = "10.7.0.1";

    /// <summary>
    /// Transforms <paramref name="backendConfigJson"/> (SOCKS inbound from
    /// <c>/vpn/connect</c>) into a config xray.exe runs with a native TUN device.
    /// </summary>
    public static string AdaptForNativeTun(string backendConfigJson)
    {
        var root = JsonNode.Parse(backendConfigJson)?.AsObject()
            ?? throw new ArgumentException("INVALID XRAY CONFIG", nameof(backendConfigJson));

        root.Remove("fakedns");
        root["dns"] = BuildDns();
        root["inbounds"] = new JsonArray(CreateTunInbound());
        EnhanceOutbounds(root);
        root["routing"] = BuildRouting();

        return root.ToJsonString(new JsonSerializerOptions { WriteIndented = true });
    }

    private static JsonObject BuildDns() => new()
    {
        ["queryStrategy"] = "UseIPv4",
        ["servers"] = new JsonArray(
            "https://1.1.1.1/dns-query",
            "https://8.8.8.8/dns-query"),
    };

    private static JsonObject CreateTunInbound() => new()
    {
        ["port"] = 0,
        ["protocol"] = "tun",
        ["tag"] = "tun-in",
        ["settings"] = new JsonObject
        {
            ["name"] = TunAdapterName,
            ["mtu"] = TunMtu,
            ["gateway"] = new JsonArray($"{TunGateway}/24"),
            ["autoOutboundsInterface"] = "auto",
        },
        ["sniffing"] = new JsonObject
        {
            ["enabled"] = true,
            ["destOverride"] = new JsonArray("http", "tls"),
            ["routeOnly"] = false,
        },
    };

    private static void EnhanceOutbounds(JsonObject root)
    {
        if (root["outbounds"] is not JsonArray outbounds) return;

        var kept = new JsonArray();
        foreach (var ob in outbounds)
        {
            if (ob is not JsonObject outbound) continue;
            if (outbound["tag"]?.GetValue<string>() == "dns-out") continue;

            if (outbound["protocol"]?.GetValue<string>() == "vless")
            {
                SetPacketEncoding(outbound);
                TuneXhttpMux(outbound);
            }

            kept.Add(outbound.DeepClone());
        }
        root["outbounds"] = kept;
    }

    private static void SetPacketEncoding(JsonObject outbound)
    {
        if (outbound["settings"] is not JsonObject settings ||
            settings["vnext"] is not JsonArray vnext)
            return;

        foreach (var node in vnext)
        {
            if (node is not JsonObject vn || vn["users"] is not JsonArray users) continue;
            foreach (var user in users)
            {
                if (user is JsonObject u)
                    u["packetEncoding"] = "xudp";
            }
        }
    }

    private static void TuneXhttpMux(JsonObject outbound)
    {
        if (outbound["streamSettings"] is not JsonObject stream ||
            stream["network"]?.GetValue<string>() != "xhttp" ||
            stream["xhttpSettings"] is not JsonObject xhttp)
            return;

        var extra = xhttp["extra"] as JsonObject ?? new JsonObject();
        extra["xmux"] = new JsonObject
        {
            ["maxConcurrency"] = 0,
            ["maxConnections"] = 1,
            ["cMaxReuseTimes"] = 0,
            ["hMaxRequestTimes"] = "1000-5000",
            ["hMaxReusableSecs"] = "1800-3000",
            ["hKeepAlivePeriod"] = 0,
        };
        xhttp["extra"] = extra;
    }

    /// <summary>
    /// Mirrors Android routing: private ranges direct (excluding 10.0.0.0/8 — the
    /// TUN subnet lives there), everything else via the VLESS outbound.
    /// </summary>
    private static JsonObject BuildRouting() => new()
    {
        ["domainStrategy"] = "AsIs",
        ["rules"] = new JsonArray
        {
            new JsonObject
            {
                ["type"] = "field",
                ["outboundTag"] = "direct",
                ["ip"] = new JsonArray(
                    "127.0.0.0/8",
                    "169.254.0.0/16",
                    "172.16.0.0/12",
                    "192.168.0.0/16",
                    "::1/128",
                    "fc00::/7",
                    "fe80::/10"),
            },
            new JsonObject
            {
                ["type"] = "field",
                ["outboundTag"] = "proxy",
                ["network"] = "tcp,udp",
            },
        },
    };
}
