using System.Text.Json;
using System.Text.Json.Nodes;
using Tyrax.Core.Models;

namespace Tyrax.Tunnel;

/// <summary>
/// Builds the Xray-core JSON config consumed by <c>xray.exe</c>. Direct port of
/// the Android <c>XrayConfigBuilder</c> so client and server stay in lockstep.
///
/// <para>Topology: a local SOCKS5 inbound (127.0.0.1:<see cref="VlessParams.SocksPort"/>)
/// feeds a VLESS outbound secured with XTLS-Reality (or CDN TLS) over TCP/XHTTP.
/// On Windows the service rewrites the backend config to a native TUN inbound via
/// <see cref="XrayWindowsConfigAdapter"/>; Android still uses tun2socks.</para>
/// </summary>
public static class XrayConfigBuilder
{
    public static string Build(VlessParams cfg, IReadOnlyList<string>? splitDomains = null)
    {
        var root = new JsonObject
        {
            ["log"] = new JsonObject { ["loglevel"] = "warning" },
        };

        // ── Inbound: local SOCKS5 ────────────────────────────────────────────
        var socksInbound = new JsonObject
        {
            ["tag"] = "socks-in",
            ["listen"] = "127.0.0.1",
            ["port"] = cfg.SocksPort,
            ["protocol"] = "socks",
            ["settings"] = new JsonObject { ["auth"] = "noauth", ["udp"] = true },
            ["sniffing"] = new JsonObject
            {
                ["enabled"] = true,
                ["destOverride"] = new JsonArray("http", "tls"),
            },
        };
        root["inbounds"] = new JsonArray(socksInbound);

        // ── Outbound: VLESS + Reality/TLS ────────────────────────────────────
        var user = new JsonObject
        {
            ["id"] = cfg.UserUuid,
            ["encryption"] = "none",
            ["flow"] = cfg.Flow, // "" → XHTTP profile; "xtls-rprx-vision" → stream-one
        };

        var vnext = new JsonObject
        {
            ["address"] = cfg.NodeHost,
            ["port"] = cfg.NodePort,
            ["users"] = new JsonArray(user),
        };

        var streamSettings = new JsonObject
        {
            ["network"] = cfg.Network,
            ["security"] = cfg.Security,
        };

        if (cfg.Security == "tls")
        {
            // CDN profile: real TLS on a Cloudflare-proxied domain (hides origin IP).
            var sni = string.IsNullOrEmpty(cfg.RealitySni) ? cfg.NodeHost : cfg.RealitySni;
            streamSettings["tlsSettings"] = new JsonObject
            {
                ["serverName"] = sni,
                ["fingerprint"] = cfg.Fingerprint,
                ["allowInsecure"] = false,
                ["alpn"] = new JsonArray("h2", "http/1.1"),
            };
        }
        else
        {
            streamSettings["realitySettings"] = new JsonObject
            {
                ["show"] = false,
                ["fingerprint"] = cfg.Fingerprint,
                ["serverName"] = cfg.RealitySni,
                ["publicKey"] = cfg.RealityPublicKey,
                ["shortId"] = cfg.RealityShortId,
                ["spiderX"] = "", // empty — must match the node's Reality config
            };
        }

        if (cfg.Network == "xhttp")
        {
            // XTLS-Vision over XHTTP only works with single-connection stream-one.
            var mode = cfg.Flow == "xtls-rprx-vision" ? "stream-one" : cfg.XhttpMode;
            var xhttpSettings = new JsonObject
            {
                ["path"] = cfg.XhttpPath,
                ["mode"] = mode,
                ["extra"] = new JsonObject { ["xPaddingBytes"] = cfg.XPaddingBytes },
            };
            // For the CDN/TLS profile the Host header must be the proxied domain.
            if (cfg.Security == "tls") xhttpSettings["host"] = cfg.NodeHost;
            streamSettings["xhttpSettings"] = xhttpSettings;
        }

        var proxyOutbound = new JsonObject
        {
            ["tag"] = "proxy",
            ["protocol"] = "vless",
            ["settings"] = new JsonObject { ["vnext"] = new JsonArray(vnext) },
            ["streamSettings"] = streamSettings,
        };

        var directOutbound = new JsonObject
        {
            ["tag"] = "direct",
            ["protocol"] = "freedom",
            ["settings"] = new JsonObject(),
        };

        var blockOutbound = new JsonObject
        {
            ["tag"] = "block",
            ["protocol"] = "blackhole",
            ["settings"] = new JsonObject(),
        };

        root["outbounds"] = new JsonArray(proxyOutbound, directOutbound, blockOutbound);

        // ── Routing ──────────────────────────────────────────────────────────
        // Keep private / LAN ranges direct so the tunnel never proxies local
        // traffic. Explicit CIDRs are used instead of "geoip:private" to avoid a
        // hard dependency on geoip.dat for this base rule.
        var rules = new JsonArray
        {
            new JsonObject
            {
                ["type"] = "field",
                ["outboundTag"] = "direct",
                ["ip"] = new JsonArray(
                    "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
                    "172.16.0.0/12", "192.168.0.0/16", "::1/128", "fc00::/7", "fe80::/10"),
            },
        };

        // Split tunnel: RU services bypass the PROTOCOL and exit directly. Uses the
        // bundled geosite.dat category plus any explicit domains from the backend.
        if (splitDomains is { Count: > 0 })
        {
            var domains = new JsonArray { "geosite:category-ru" };
            foreach (var d in splitDomains) domains.Add($"domain:{d}");
            rules.Add(new JsonObject
            {
                ["type"] = "field",
                ["outboundTag"] = "direct",
                ["domain"] = domains,
            });
        }

        root["routing"] = new JsonObject
        {
            ["domainStrategy"] = "IPIfNonMatch",
            ["rules"] = rules,
        };

        return root.ToJsonString(new JsonSerializerOptions { WriteIndented = false });
    }
}
