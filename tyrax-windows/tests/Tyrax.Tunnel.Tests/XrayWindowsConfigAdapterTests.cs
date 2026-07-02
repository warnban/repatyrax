using System.Text.Json;
using Tyrax.Tunnel;
using Xunit;

namespace Tyrax.Tunnel.Tests;

public sealed class XrayWindowsConfigAdapterTests
{
    private const string BackendSample = """
        {
          "log": { "loglevel": "warning" },
          "fakedns": [{ "ipPool": "198.18.0.0/15", "poolSize": 65535 }],
          "dns": {
            "queryStrategy": "UseIPv4",
            "servers": ["fakedns", { "address": "1.1.1.1", "port": 53, "proxyTag": "proxy" }]
          },
          "inbounds": [{
            "tag": "socks-in",
            "listen": "127.0.0.1",
            "port": 10808,
            "protocol": "socks",
            "settings": { "auth": "noauth", "udp": true },
            "sniffing": { "enabled": true, "destOverride": ["fakedns", "http", "tls", "quic"] }
          }],
          "outbounds": [
            {
              "tag": "proxy",
              "protocol": "vless",
              "settings": { "vnext": [{ "address": "node.example", "port": 443, "users": [{ "id": "uuid" }] }] },
              "streamSettings": {
                "network": "xhttp",
                "security": "reality",
                "xhttpSettings": { "path": "/api/v1/data", "mode": "auto", "extra": { "xPaddingBytes": "100-1000" } }
              }
            },
            { "tag": "direct", "protocol": "freedom", "settings": {} },
            { "tag": "dns-out", "protocol": "dns", "settings": {} }
          ],
          "routing": {
            "domainStrategy": "IPIfNonMatch",
            "rules": [
              { "type": "field", "inboundTag": ["socks-in"], "port": "53", "network": "tcp,udp", "outboundTag": "dns-out" },
              { "type": "field", "outboundTag": "proxy", "ip": ["198.18.0.0/15", "240.0.0.0/4"] },
              { "type": "field", "outboundTag": "direct", "ip": ["127.0.0.0/8"] },
              { "type": "field", "outboundTag": "proxy", "network": "tcp,udp" }
            ]
          }
        }
        """;

    [Fact]
    public void AdaptForNativeTun_ReplacesSocksWithTunAndStripsOverhead()
    {
        var adapted = XrayWindowsConfigAdapter.AdaptForNativeTun(BackendSample);
        using var doc = JsonDocument.Parse(adapted);
        var root = doc.RootElement;

        Assert.False(root.TryGetProperty("fakedns", out _));

        var inbound = root.GetProperty("inbounds")[0];
        Assert.Equal("tun", inbound.GetProperty("protocol").GetString());
        Assert.Equal("TYRAX", inbound.GetProperty("settings").GetProperty("name").GetString());
        Assert.Equal(1400, inbound.GetProperty("settings").GetProperty("mtu").GetInt32());
        Assert.Equal("10.7.0.1/24", inbound.GetProperty("settings").GetProperty("gateway")[0].GetString());
        Assert.Equal("auto", inbound.GetProperty("settings").GetProperty("autoOutboundsInterface").GetString());
        Assert.True(inbound.GetProperty("sniffing").GetProperty("enabled").GetBoolean());

        var dnsServers = root.GetProperty("dns").GetProperty("servers");
        Assert.Equal(2, dnsServers.GetArrayLength());
        Assert.Equal("https://1.1.1.1/dns-query", dnsServers[0].GetString());

        var tags = root.GetProperty("outbounds").EnumerateArray()
            .Select(o => o.GetProperty("tag").GetString()).ToArray();
        Assert.DoesNotContain("dns-out", tags);

        var proxyUser = root.GetProperty("outbounds")[0]
            .GetProperty("settings").GetProperty("vnext")[0]
            .GetProperty("users")[0];
        Assert.Equal("xudp", proxyUser.GetProperty("packetEncoding").GetString());

        var xmux = root.GetProperty("outbounds")[0]
            .GetProperty("streamSettings").GetProperty("xhttpSettings")
            .GetProperty("extra").GetProperty("xmux");
        Assert.Equal(1, xmux.GetProperty("maxConnections").GetInt32());

        var routing = root.GetProperty("routing");
        Assert.Equal("AsIs", routing.GetProperty("domainStrategy").GetString());
        var directRule = routing.GetProperty("rules")[0];
        var directIps = directRule.GetProperty("ip").EnumerateArray().Select(i => i.GetString()).ToArray();
        Assert.DoesNotContain(directIps, ip => ip!.StartsWith("10."));

        Assert.Equal("node.example", XrayConfigInspector.GetProxyHost(adapted));
    }
}
