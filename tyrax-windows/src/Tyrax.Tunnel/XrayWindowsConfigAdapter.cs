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

    /// <summary>
    /// TUN device MTU fed to the xray TUN inbound. Lowered 1400 → 1280 so the
    /// inner IP packets never exceed the effective path MTU once xhttp + Reality +
    /// TLS/H2 framing overhead is added: at 1400 the outer segments fragmented /
    /// tripped PMTUD, which made throughput slow and rough for the first minutes of
    /// a session until the path settled. 1280 (the IPv6 minimum MTU) is the safe
    /// floor that avoids fragmentation over any transit. Must stay in sync with
    /// <c>TunnelConstants.TunMtu</c> so the OS adapter and xray agree.
    /// </summary>
    public const int TunMtu = 1280;

    /// <summary>Must match <c>TunnelConstants.TunGateway</c> on the service side.</summary>
    public const string TunGateway = "10.7.0.1";

    /// <summary>
    /// Transforms <paramref name="backendConfigJson"/> (SOCKS inbound from
    /// <c>/vpn/connect</c>) into a config xray.exe runs with a native TUN device.
    /// </summary>
    public static string AdaptForNativeTun(
        string backendConfigJson,
        IReadOnlyList<string>? splitDomains = null,
        bool splitEnabled = false)
    {
        var root = JsonNode.Parse(backendConfigJson)?.AsObject()
            ?? throw new ArgumentException("INVALID XRAY CONFIG", nameof(backendConfigJson));

        root.Remove("fakedns");
        root["dns"] = BuildDns();
        root["inbounds"] = new JsonArray(CreateTunInbound());
        EnhanceOutbounds(root);
        root["routing"] = BuildRouting(splitDomains, splitEnabled);

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
            // Desktop throughput-ramp tuning. Per-connection concurrent-stream cap.
            // "16-32" (Xray's documented default) makes XmuxManager open a NEW H2
            // connection once a connection saturates that many streams, instead of
            // 0 = unlimited streams crammed onto as few connections as possible. On
            // desktop DPI throttle-avoidance is unnecessary, so spreading the TUN
            // streams across several connections lets each connection's congestion
            // window ramp in parallel → aggregate throughput reaches full speed in
            // seconds instead of ~5 min. (Android keeps single-mux anti-throttle.)
            ["maxConcurrency"] = "16-32",
            // Connection pool size. XmuxManager.GetXmuxClient() prefers creating a
            // brand-new connection while the pool is below maxConnections, so raising
            // this from 2 → 6 means the first several streams each get their own
            // connection immediately — six cwnds ramping at once. Each connection also
            // gets its own UnreusableAt = now + rand(hMaxReusableSecs), so recycles are
            // STAGGERED across six connections: the ~2–3h time-based recycle never
            // zeroes traffic (warm standby preserved, strengthening the v1.0.17 fix).
            ["maxConnections"] = 6,
            ["cMaxReuseTimes"] = 0,
            ["hMaxRequestTimes"] = "1000-5000",
            // Time-based recycle window; the 6-connection pool above staggers these so a
            // recycle never zeroes traffic.
            ["hMaxReusableSecs"] = "7200-10800",
            // HTTP/2 ReadIdleTimeout (seconds) → H2 PING keepalive interval. 0 let Xray fall
            // back to a browser-like default and the idle mux could be silently dropped by
            // NAT/carrier between recycles; a deterministic 30s PING keeps the connection warm
            // and surfaces a dead peer quickly.
            ["hKeepAlivePeriod"] = 30,
        };
        xhttp["extra"] = extra;
    }

    /// <summary>
    /// Mirrors Android routing: private ranges direct (excluding 10.0.0.0/8 — the
    /// TUN subnet lives there), everything else via the VLESS outbound.
    ///
    /// <para>When <paramref name="splitEnabled"/>, RU traffic is bypassed DIRECTLY
    /// (<c>geosite:category-ru</c> + explicit <c>domain:</c> rules + <c>geoip:ru</c>)
    /// before the proxy catch-all, so RU-geoblocked apps (banks, Wildberries, Ozon)
    /// reach their servers over the real network with the tunnel on — parity with
    /// the Android <c>XrayConfigPatcher</c>. Requires geoip.dat/geosite.dat beside
    /// xray.exe. <c>domainStrategy=IPIfNonMatch</c> lets the geoip rule catch RU IPs
    /// even when the request is by domain.</para>
    /// </summary>
    private static JsonObject BuildRouting(IReadOnlyList<string>? splitDomains, bool splitEnabled)
    {
        var rules = new JsonArray
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
        };

        if (splitEnabled)
        {
            // Cast to JsonNode so string values become primitive nodes (matching the
            // constructor form used elsewhere); the generic Add<string> overload would
            // create a customized value that fails ToJsonString with custom options.
            var domains = new JsonArray { (JsonNode)"geosite:category-ru" };
            if (splitDomains is { Count: > 0 })
                foreach (var d in splitDomains) domains.Add((JsonNode)$"domain:{d}");

            rules.Add(new JsonObject
            {
                ["type"] = "field",
                ["outboundTag"] = "direct",
                ["domain"] = domains,
            });
            rules.Add(new JsonObject
            {
                ["type"] = "field",
                ["outboundTag"] = "direct",
                ["ip"] = new JsonArray("geoip:ru"),
            });
        }

        rules.Add(new JsonObject
        {
            ["type"] = "field",
            ["outboundTag"] = "proxy",
            ["network"] = "tcp,udp",
        });

        return new JsonObject
        {
            ["domainStrategy"] = splitEnabled ? "IPIfNonMatch" : "AsIs",
            ["rules"] = rules,
        };
    }
}
