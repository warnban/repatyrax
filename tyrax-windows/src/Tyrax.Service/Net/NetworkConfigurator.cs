using System.Diagnostics;
using System.Globalization;
using System.Linq;
using System.Net;
using System.Net.NetworkInformation;
using System.Net.Sockets;
using System.Runtime.InteropServices;
using System.Text;
using Microsoft.Extensions.Logging;
using Tyrax.Service.Engines;

namespace Tyrax.Service.Net;

/// <summary>
/// Installs and tears down the OS-level networking that turns the WinTun adapter
/// into the system default route:
/// <list type="number">
///   <item>pin a <c>/32</c> route to the node via the real gateway (breaks the loop),</item>
///   <item>address the TUN adapter and point its DNS through the tunnel,</item>
///   <item>add split-default routes (<c>0.0.0.0/1</c> + <c>128.0.0.0/1</c>) via the TUN,
///         overriding the physical default without deleting it,</item>
///   <item>pin RU split-tunnel domains as <c>/32</c> routes via the real gateway so
///         those services see the user's real (RU) IP and bypass the tunnel.</item>
/// </list>
/// Domain split reuses the node-pin trick: a resolved RU IP routed via the physical
/// gateway never enters the TUN, so it goes direct without an xray routing loop —
/// the Windows equivalent of the Android per-app RU exclusion. All mutations require
/// the service to run elevated (LocalSystem in production).
/// </summary>
public sealed class NetworkConfigurator
{
    private const int MaxSplitDomains = 128;
    private static readonly TimeSpan ResolveTimeout = TimeSpan.FromSeconds(2);

    // netsh / route write to the console in the OEM codepage (e.g. cp866 on a
    // Russian Windows). Decode with it so logged error messages are readable
    // instead of mojibake; fall back to UTF-8 if the codepage is unavailable.
    private static readonly Encoding OemEncoding = ResolveOemEncoding();

    private static Encoding ResolveOemEncoding()
    {
        try
        {
            Encoding.RegisterProvider(CodePagesEncodingProvider.Instance);
            return Encoding.GetEncoding(CultureInfo.CurrentCulture.TextInfo.OEMCodePage);
        }
        catch
        {
            return Encoding.UTF8;
        }
    }

    private readonly ILogger _logger;
    private IPAddress? _nodeIp;
    private IPAddress? _gwIp;
    private int _gwIfIndex;
    private readonly HashSet<string> _splitIps = new();
    private bool _applied;

    public NetworkConfigurator(ILogger logger) => _logger = logger;

    /// <summary>Resolves the node, pins its route, configures the TUN, default routes and RU split.</summary>
    public async Task ApplyAsync(string nodeHost, IReadOnlyList<string> splitDomains, CancellationToken ct)
    {
        _nodeIp = await ResolveIPv4Async(nodeHost, ct);
        if (_nodeIp is null)
            throw new InvalidOperationException($"CANNOT RESOLVE NODE {nodeHost}");

        // 1. Pin the node through the current physical default route so xray's own
        //    connection to the node never re-enters the tunnel.
        var (gwIp, gwIfIndex) = GetPhysicalGateway(_nodeIp);
        if (gwIp is null)
            throw new InvalidOperationException("NO PHYSICAL GATEWAY");
        _gwIp = gwIp;
        _gwIfIndex = gwIfIndex;

        await RunAsync("route", $"add {_nodeIp} mask 255.255.255.255 {gwIp} metric 1 if {gwIfIndex}", ct);

        // 2. The TUN adapter is created by xray; wait for it, then address it.
        //    The interface index shows up a moment before the IP stack / alias is
        //    ready to accept configuration, so netsh briefly returns "element not
        //    found" (exit 1). Retry the interface commands until they take.
        var tunIndex = await WaitForAdapterAsync(TunnelConstants.AdapterName, ct);

        await RunWithRetryAsync("netsh",
            $"interface ipv4 set address name=\"{TunnelConstants.AdapterName}\" static {TunnelConstants.TunAddress} {TunnelConstants.TunMask}", ct);
        await RunWithRetryAsync("netsh",
            $"interface ipv4 set dns name=\"{TunnelConstants.AdapterName}\" static {TunnelConstants.PrimaryDns} primary", ct);
        await RunWithRetryAsync("netsh",
            $"interface ipv4 add dns name=\"{TunnelConstants.AdapterName}\" {TunnelConstants.SecondaryDns} index=2", ct);

        // 3. Split-default routes take precedence over the physical /0 by being more
        //    specific, so we never have to remove (and later restore) the real one.
        await RunAsync("route", $"add 0.0.0.0 mask 128.0.0.0 {TunnelConstants.TunGateway} metric 1 if {tunIndex}", ct);
        await RunAsync("route", $"add 128.0.0.0 mask 128.0.0.0 {TunnelConstants.TunGateway} metric 1 if {tunIndex}", ct);

        _applied = true;
        _logger.LogInformation("routes up: node {Node} pinned via {Gw}, default via TUN {Idx}", _nodeIp, gwIp, tunIndex);

        // 4. RU split: pin each resolved domain IP via the physical gateway (bypass TUN).
        await PinSplitDomainsAsync(splitDomains, ct);
    }

    /// <summary>
    /// Re-wires the WinTun adapter after an in-place xray restart: the adapter name is
    /// stable but the interface index and split-default routes must be reinstalled.
    /// Node and RU split <c>/32</c> pins are refreshed via <see cref="RepinNodeAsync"/>.
    /// </summary>
    public async Task ReestablishTunRoutesAsync(CancellationToken ct)
    {
        if (!_applied || _nodeIp is null)
            throw new InvalidOperationException("NETWORK NOT APPLIED");

        await RunAsync("route", "delete 0.0.0.0 mask 128.0.0.0", ct, ignoreErrors: true);
        await RunAsync("route", "delete 128.0.0.0 mask 128.0.0.0", ct, ignoreErrors: true);

        var tunIndex = await WaitForAdapterAsync(TunnelConstants.AdapterName, ct);

        await RunWithRetryAsync("netsh",
            $"interface ipv4 set address name=\"{TunnelConstants.AdapterName}\" static {TunnelConstants.TunAddress} {TunnelConstants.TunMask}", ct);
        await RunWithRetryAsync("netsh",
            $"interface ipv4 set dns name=\"{TunnelConstants.AdapterName}\" static {TunnelConstants.PrimaryDns} primary", ct);
        await RunWithRetryAsync("netsh",
            $"interface ipv4 add dns name=\"{TunnelConstants.AdapterName}\" {TunnelConstants.SecondaryDns} index=2", ct,
            ignoreErrorsOnLast: true);

        await RunAsync("route", $"add 0.0.0.0 mask 128.0.0.0 {TunnelConstants.TunGateway} metric 1 if {tunIndex}", ct);
        await RunAsync("route", $"add 128.0.0.0 mask 128.0.0.0 {TunnelConstants.TunGateway} metric 1 if {tunIndex}", ct);

        if (!await RepinNodeAsync(ct))
            throw new InvalidOperationException("NODE RE-PIN FAILED AFTER XRAY RESTART");

        _logger.LogInformation("TUN routes re-established on adapter index {Idx}", tunIndex);
    }

    /// <summary>
    /// Re-pins the node <c>/32</c> and all RU split routes through the CURRENT physical
    /// gateway. Called on a network handoff (Wi-Fi ↔ Ethernet ↔ LTE): the gateway that
    /// reaches them may have changed, which would otherwise strand the tunnel or leak
    /// RU traffic. The TUN default routes are unaffected. Returns false if it could not re-pin.
    /// </summary>
    public async Task<bool> RepinNodeAsync(CancellationToken ct)
    {
        if (!_applied || _nodeIp is null) return false;

        var (gwIp, gwIfIndex) = GetPhysicalGateway(_nodeIp);
        if (gwIp is null) return false;
        _gwIp = gwIp;
        _gwIfIndex = gwIfIndex;

        await RunAsync("route", $"delete {_nodeIp} mask 255.255.255.255", ct, ignoreErrors: true);
        try
        {
            await RunAsync("route", $"add {_nodeIp} mask 255.255.255.255 {gwIp} metric 1 if {gwIfIndex}", ct);
        }
        catch (Exception ex)
        {
            _logger.LogWarning(ex, "re-pin failed");
            return false;
        }

        // Re-pin RU split /32s via the new gateway (best-effort; a stale RU route only
        // means that service momentarily goes through the tunnel, not a hard failure).
        foreach (var ip in _splitIps.ToArray())
        {
            await RunAsync("route", $"delete {ip} mask 255.255.255.255", ct, ignoreErrors: true);
            await RunAsync("route", $"add {ip} mask 255.255.255.255 {gwIp} metric 1 if {gwIfIndex}", ct, ignoreErrors: true);
        }

        _logger.LogInformation("node {Node} + {Count} RU routes re-pinned via {Gw} after handoff", _nodeIp, _splitIps.Count, gwIp);
        return true;
    }

    /// <summary>Removes everything <see cref="ApplyAsync"/> installed. Safe to call twice.</summary>
    public async Task RevertAsync(CancellationToken ct)
    {
        if (!_applied) return;
        _applied = false;

        await RunAsync("route", "delete 0.0.0.0 mask 128.0.0.0", ct, ignoreErrors: true);
        await RunAsync("route", "delete 128.0.0.0 mask 128.0.0.0", ct, ignoreErrors: true);
        if (_nodeIp is not null)
            await RunAsync("route", $"delete {_nodeIp} mask 255.255.255.255", ct, ignoreErrors: true);

        foreach (var ip in _splitIps)
            await RunAsync("route", $"delete {ip} mask 255.255.255.255", ct, ignoreErrors: true);
        _splitIps.Clear();

        _logger.LogInformation("routes reverted");
    }

    // ── RU split ─────────────────────────────────────────────────────────────

    private async Task PinSplitDomainsAsync(IReadOnlyList<string> domains, CancellationToken ct)
    {
        if (_gwIp is null || domains.Count == 0) return;

        var hosts = domains.Take(MaxSplitDomains).ToArray();
        var results = await Task.WhenAll(hosts.Select(h => ResolveAllIPv4Async(h, ct)));

        var pinned = 0;
        foreach (var ip in results.SelectMany(r => r).Distinct(StringComparer.Ordinal))
        {
            if (ip == _nodeIp?.ToString()) continue; // never override the node pin
            if (!_splitIps.Add(ip)) continue;
            await RunAsync("route", $"add {ip} mask 255.255.255.255 {_gwIp} metric 1 if {_gwIfIndex}", ct, ignoreErrors: true);
            pinned++;
        }
        _logger.LogInformation("RU split: pinned {Pinned} IPs from {Domains} domains via {Gw}", pinned, hosts.Length, _gwIp);
    }

    private async Task<IReadOnlyList<string>> ResolveAllIPv4Async(string host, CancellationToken ct)
    {
        if (IPAddress.TryParse(host, out var literal))
            return literal.AddressFamily == AddressFamily.InterNetwork ? new[] { literal.ToString() } : Array.Empty<string>();
        try
        {
            using var timeout = CancellationTokenSource.CreateLinkedTokenSource(ct);
            timeout.CancelAfter(ResolveTimeout);
            var addrs = await Dns.GetHostAddressesAsync(host, timeout.Token);
            return addrs.Where(a => a.AddressFamily == AddressFamily.InterNetwork)
                        .Select(a => a.ToString()).ToArray();
        }
        catch (Exception)
        {
            return Array.Empty<string>();
        }
    }

    // ── helpers ──────────────────────────────────────────────────────────────

    private static async Task<IPAddress?> ResolveIPv4Async(string host, CancellationToken ct)
    {
        if (IPAddress.TryParse(host, out var literal))
            return literal.AddressFamily == AddressFamily.InterNetwork ? literal : null;

        var addrs = await Dns.GetHostAddressesAsync(host, ct);
        return Array.Find(addrs, a => a.AddressFamily == AddressFamily.InterNetwork);
    }

    /// <summary>
    /// Finds the interface/gateway that currently reaches <paramref name="dest"/>
    /// via <c>GetBestInterface</c>, then reads that interface's IPv4 gateway.
    /// </summary>
    private (IPAddress? gateway, int ifIndex) GetPhysicalGateway(IPAddress dest)
    {
        uint destAddr = BitConverter.ToUInt32(dest.GetAddressBytes(), 0);
        if (GetBestInterface(destAddr, out uint ifIndex) != 0)
            return (null, 0);

        foreach (var nic in NetworkInterface.GetAllNetworkInterfaces())
        {
            if (nic.OperationalStatus != OperationalStatus.Up) continue;
            var props = nic.GetIPProperties();
            int idx;
            try { idx = props.GetIPv4Properties().Index; }
            catch (NetworkInformationException) { continue; }
            if (idx != (int)ifIndex) continue;

            foreach (var gw in props.GatewayAddresses)
            {
                if (gw.Address.AddressFamily == AddressFamily.InterNetwork &&
                    !gw.Address.Equals(IPAddress.Any))
                {
                    return (gw.Address, idx);
                }
            }
        }
        return (null, (int)ifIndex);
    }

    private async Task<int> WaitForAdapterAsync(string name, CancellationToken ct)
    {
        var deadline = DateTime.UtcNow.AddSeconds(10);
        while (DateTime.UtcNow < deadline)
        {
            ct.ThrowIfCancellationRequested();
            foreach (var nic in NetworkInterface.GetAllNetworkInterfaces())
            {
                if (!string.Equals(nic.Name, name, StringComparison.OrdinalIgnoreCase)) continue;
                try { return nic.GetIPProperties().GetIPv4Properties().Index; }
                catch (NetworkInformationException) { /* not ready yet */ }
            }
            await Task.Delay(200, ct);
        }
        throw new TimeoutException($"TUN ADAPTER {name} DID NOT APPEAR");
    }

    /// <summary>
    /// Runs a command, retrying on non-zero exit. Used for the WinTun interface
    /// setup, which races the adapter's IP-stack initialisation right after
    /// creation (netsh answers "element not found" for a fraction of a second).
    /// </summary>
    private Task RunWithRetryAsync(string file, string args, CancellationToken ct,
        int attempts = 15, int delayMs = 300, bool ignoreErrorsOnLast = false)
        => RunWithRetryCoreAsync(file, args, ct, attempts, delayMs, ignoreErrorsOnLast);

    private async Task RunWithRetryCoreAsync(string file, string args, CancellationToken ct,
        int attempts, int delayMs, bool ignoreErrorsOnLast)
    {
        for (int i = 1; ; i++)
        {
            var last = i >= attempts;
            try
            {
                await RunAsync(file, args, ct, ignoreErrors: last && ignoreErrorsOnLast);
                if (i > 1) _logger.LogInformation("{File} {Args} succeeded on attempt {I}", file, args, i);
                return;
            }
            catch (Exception ex) when (!last && !ct.IsCancellationRequested)
            {
                _logger.LogDebug("attempt {I}/{N} failed ({Msg}); retrying", i, attempts, ex.Message);
                await Task.Delay(delayMs, ct);
            }
        }
    }

    private async Task RunAsync(string file, string args, CancellationToken ct, bool ignoreErrors = false)
    {
        var psi = new ProcessStartInfo
        {
            FileName = file,
            Arguments = args,
            UseShellExecute = false,
            CreateNoWindow = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            StandardOutputEncoding = OemEncoding,
            StandardErrorEncoding = OemEncoding,
        };
        using var proc = Process.Start(psi) ?? throw new InvalidOperationException($"CANNOT RUN {file}");
        string stdout = await proc.StandardOutput.ReadToEndAsync(ct);
        string stderr = await proc.StandardError.ReadToEndAsync(ct);
        await proc.WaitForExitAsync(ct);

        if (proc.ExitCode != 0 && !ignoreErrors)
        {
            throw new InvalidOperationException(
                $"{file} {args} -> exit {proc.ExitCode}: {stdout.Trim()} {stderr.Trim()}");
        }
        _logger.LogDebug("{File} {Args} -> {Code}", file, args, proc.ExitCode);
    }

    [DllImport("iphlpapi.dll", SetLastError = false)]
    private static extern int GetBestInterface(uint dwDestAddr, out uint pdwBestIfIndex);
}
