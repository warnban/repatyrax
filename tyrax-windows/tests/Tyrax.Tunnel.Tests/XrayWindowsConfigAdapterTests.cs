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
            { "tag": "proxy", "protocol": "vless", "settings": { "vnext": [{ "address": "node.example", "port": 443, "users": [{ "id": "uuid" }] }] } },
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
        Assert.False(inbound.TryGetProperty("sniffing", out _));

        var dnsServers = root.GetProperty("dns").GetProperty("servers");
        Assert.Equal(1, dnsServers.GetArrayLength());
        Assert.Equal("1.1.1.1", dnsServers[0].GetProperty("address").GetString());

        var tags = root.GetProperty("outbounds").EnumerateArray()
            .Select(o => o.GetProperty("tag").GetString()).ToArray();
        Assert.DoesNotContain("dns-out", tags);

        var rules = root.GetProperty("routing").GetProperty("rules");
        Assert.All(rules.EnumerateArray(), r =>
        {
            if (r.TryGetProperty("outboundTag", out var tag))
                Assert.NotEqual("dns-out", tag.GetString());
            if (r.TryGetProperty("ip", out var ips))
                Assert.All(ips.EnumerateArray(), ip =>
                    Assert.DoesNotContain("198.18", ip.GetString()));
        });

        Assert.Equal("node.example", XrayConfigInspector.GetProxyHost(adapted));
    }
}
