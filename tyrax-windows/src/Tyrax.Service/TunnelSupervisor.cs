using System.Net.NetworkInformation;
using Microsoft.Extensions.Logging;
using Tyrax.Core.Models;
using Tyrax.Ipc;
using Tyrax.Service.Engines;
using Tyrax.Service.Net;
using Tyrax.Tunnel;

namespace Tyrax.Service;

/// <summary>
/// Owns the PROTOCOL lifecycle on the privileged side. Brings the tunnel up by
/// (1) adapting the backend config for native TUN, (2) starting <c>xray.exe</c>
/// (WinTun inbound), (3) wiring routes/DNS — and tears it
/// all down in reverse. Connect/disconnect are serialized by a semaphore.
///
/// <para>While connected, a monitor loop watches the engines. Transient health blips
/// (e.g. XHTTP mux recycle) trigger an in-place xray restart before a full degrade;
/// only sustained failure or an unrecoverable engine crash ends in
/// <see cref="TunnelState.Error"/> for the UI supervisor to switch node.</para>
/// </summary>
public sealed class TunnelSupervisor : IAsyncDisposable
{
    // Generous grace + interval so a busy-but-live tunnel is never mistaken for
    // dead; only a sustained run of failures triggers a degrade. Mirrors Android.
    private static readonly TimeSpan Grace = TimeSpan.FromSeconds(12);
    private static readonly TimeSpan Tick = TimeSpan.FromSeconds(5);
    private static readonly TimeSpan ProbeInterval = TimeSpan.FromSeconds(30);
    // Desktop links to distant nodes (FI/PL) routinely need more than 9s for a cold
    // TLS probe through xhttp+reality; Android keeps 9s on mobile where the supervisor
    // shares the same radio as the tunnel. 18s avoids false NODE DEGRADED on Windows.
    private const long ThrottleMs = 18_000;
    private const int MaxFails = 4;
    private const int MaxEngineRecoveries = 2;

    private readonly ILogger<TunnelSupervisor> _logger;
    private readonly SemaphoreSlim _gate = new(1, 1);
    private readonly object _stateLock = new();

    private EnginePaths? _paths;
    private XrayProcess? _xray;
    private NetworkConfigurator? _net;
    private CancellationTokenSource? _monitorCts;
    private NetworkAddressChangedEventHandler? _netChangeHandler;

    private string? _adaptedConfigJson;
    private int _engineRecoveryAttempts;

    private TunnelState _state = TunnelState.Disconnected;
    private string? _codename;
    private string? _message;
    private long _txBytes;
    private long _rxBytes;
    public TunnelSupervisor(ILogger<TunnelSupervisor> logger) => _logger = logger;

    /// <summary>Raised whenever the tunnel status changes, so the IPC server can push it.</summary>
    public event Action<IpcStatus>? StatusChanged;

    public IpcStatus Snapshot()
    {
        lock (_stateLock)
        {
            return new IpcStatus
            {
                State = _state,
                Codename = _codename,
                Message = _message,
                TxBytes = _txBytes,
                RxBytes = _rxBytes,
            };
        }
    }

    public async Task<IpcStatus> ConnectAsync(IpcCommand cmd, CancellationToken ct)
    {
        if (string.IsNullOrEmpty(cmd.ConfigJson) || string.IsNullOrEmpty(cmd.Codename))
            return Set(TunnelState.Error, null, "INVALID CONFIG");

        var nodeHost = XrayConfigInspector.GetProxyHost(cmd.ConfigJson);
        if (string.IsNullOrEmpty(nodeHost))
            return Set(TunnelState.Error, null, "INVALID CONFIG");

        await _gate.WaitAsync(ct);
        try
        {
            await TeardownAsync(ct); // ensure a clean slate
            Set(TunnelState.Connecting, cmd.Codename, null);
            _logger.LogInformation("BREACHING NETWORK: node {Codename} ({Host})", cmd.Codename, nodeHost);

            _paths ??= new EnginePaths();

            // RU split-tunnel defaults ON (parity with Android): when the UI supplies a
            // split list (it falls back to SplitTunnelDefaults.RuDomains), RU traffic is
            // routed direct in the engine config so geoblocked apps bypass the node.
            var splitDomains = cmd.SplitDomains;
            var splitEnabled = splitDomains is { Count: > 0 };
            var configJson = XrayWindowsConfigAdapter.AdaptForNativeTun(cmd.ConfigJson, splitDomains, splitEnabled);
            _adaptedConfigJson = configJson;
            _engineRecoveryAttempts = 0;
            _xray = new XrayProcess(_paths, _logger);
            await _xray.StartAsync(configJson, ct);

            _net = new NetworkConfigurator(_logger);
            await _net.ApplyAsync(nodeHost, cmd.SplitDomains ?? Array.Empty<string>(), ct);

            // The old OS resolver cache still holds pre-tunnel (often RU-geo / negative)
            // answers; drop it so lookups resolve through the tunnel immediately.
            FlushDnsCache();

            var status = Set(TunnelState.Connected, cmd.Codename, null);
            StartMonitors();

            // Kick the xray outbound so the VLESS+Reality+XHTTP handshake happens now
            // instead of on the user's first page load (cuts the "works after ~minute").
            _ = Task.Run(() => WarmUpAsync(_monitorCts?.Token ?? CancellationToken.None));
            return status;
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "BREACH FAILED");
            await TeardownAsync(CancellationToken.None);
            var message = ex is InvalidOperationException or TimeoutException
                ? ex.Message
                : "CONNECTION FAILED. NODE UNAVAILABLE.";
            return Set(TunnelState.Error, null, message);
        }
        finally
        {
            _gate.Release();
        }
    }

    public async Task<IpcStatus> DisconnectAsync(CancellationToken ct)
    {
        await _gate.WaitAsync(ct);
        try
        {
            Set(TunnelState.Disconnecting, _codename, null);
            _logger.LogInformation("LEAVING SYSTEM");
            await TeardownAsync(ct);
            return Set(TunnelState.Disconnected, null, null);
        }
        finally
        {
            _gate.Release();
        }
    }

    // ── Monitoring: stats + watchdog + health + handoff ────────────────────────

    private void StartMonitors()
    {
        _monitorCts = new CancellationTokenSource();
        var token = _monitorCts.Token;

        _ = Task.Run(() => StatsLoopAsync(token), token);
        _ = Task.Run(() => MonitorLoopAsync(token), token);

        _netChangeHandler = (_, _) => OnNetworkChanged();
        NetworkChange.NetworkAddressChanged += _netChangeHandler;
    }

    private void StopMonitors()
    {
        if (_netChangeHandler is not null)
        {
            NetworkChange.NetworkAddressChanged -= _netChangeHandler;
            _netChangeHandler = null;
        }
        _monitorCts?.Cancel();
        _monitorCts?.Dispose();
        _monitorCts = null;
    }

    private async Task StatsLoopAsync(CancellationToken token)
    {
        while (!token.IsCancellationRequested)
        {
            if (NetworkStats.TryRead(TunnelConstants.AdapterName, out var tx, out var rx))
                PushStats(tx, rx);
            try { await Task.Delay(1000, token); }
            catch (OperationCanceledException) { break; }
        }
    }

    /// <summary>
    /// Watches the live tunnel. A dead engine or a sustained health failure ends
    /// the tunnel in Error so the UI supervisor can switch node. Runs until the
    /// tunnel leaves Connected or monitoring is cancelled.
    /// </summary>
    private async Task MonitorLoopAsync(CancellationToken token)
    {
        try { await Task.Delay(Grace, token); }
        catch (OperationCanceledException) { return; }

        var fails = 0;
        var sinceProbe = TimeSpan.Zero;

        while (!token.IsCancellationRequested)
        {
            lock (_stateLock) { if (_state != TunnelState.Connected) return; }

            // Watchdog: try in-place recovery before a full degrade.
            if (_xray is { IsRunning: false })
            {
                _logger.LogWarning("xray died → attempting recovery");
                if (await TryRecoverEngineAsync(token))
                {
                    fails = 0;
                    continue;
                }
                _ = DegradeAsync("ENGINE FAILED");
                return;
            }

            sinceProbe += Tick;
            if (sinceProbe >= ProbeInterval)
            {
                sinceProbe = TimeSpan.Zero;
                if (fails > 0)
                    TunnelHealth.Reset();
                var result = await TunnelHealth.ProbeAsync(token);
                if (!result.Ok || result.ElapsedMs > ThrottleMs) fails++;
                else fails = 0;
                _logger.LogDebug("health ok={Ok} took={Ms}ms fails={Fails}", result.Ok, result.ElapsedMs, fails);

                if (fails >= MaxFails)
                {
                    _logger.LogWarning("health degraded ({Fails} fails) → attempting recovery", fails);
                    if (await TryRecoverEngineAsync(token))
                    {
                        fails = 0;
                        continue;
                    }
                    _ = DegradeAsync("NODE DEGRADED");
                    return;
                }
            }

            try { await Task.Delay(Tick, token); }
            catch (OperationCanceledException) { return; }
        }
    }

    /// <summary>
    /// Restarts xray in-place (routes stay pinned) after mux recycle or a dead process.
    /// Keeps the user connected without forcing the UI supervisor to switch node.
    /// </summary>
    private async Task<bool> TryRecoverEngineAsync(CancellationToken token)
    {
        if (_adaptedConfigJson is null || _paths is null || _net is null)
            return false;
        if (_engineRecoveryAttempts >= MaxEngineRecoveries)
            return false;

        await _gate.WaitAsync(token);
        try
        {
            lock (_stateLock) { if (_state != TunnelState.Connected) return false; }

            _engineRecoveryAttempts++;
            _logger.LogInformation("in-place xray recovery attempt {N}/{Max}",
                _engineRecoveryAttempts, MaxEngineRecoveries);

            TunnelHealth.Reset();

            if (_xray is not null)
            {
                await _xray.DisposeAsync();
                _xray = null;
            }

            _xray = new XrayProcess(_paths, _logger);
            await _xray.StartAsync(_adaptedConfigJson, token);
            await _net.ReestablishTunRoutesAsync(token);
            FlushDnsCache();

            for (var i = 0; i < 4; i++)
            {
                lock (_stateLock) { if (_state != TunnelState.Connected) return false; }
                var result = await TunnelHealth.ProbeAsync(token);
                if (result.Ok)
                {
                    _logger.LogInformation("engine recovered in ~{Ms}ms", result.ElapsedMs);
                    return true;
                }
                try { await Task.Delay(2000, token); }
                catch (OperationCanceledException) { return false; }
            }

            _logger.LogWarning("engine recovery: xray up but health probe still failing");
            if (_xray is not null)
            {
                await _xray.DisposeAsync();
                _xray = null;
            }
            return false;
        }
        catch (Exception ex)
        {
            _logger.LogWarning(ex, "engine recovery failed");
            if (_xray is not null)
            {
                try { await _xray.DisposeAsync(); } catch { /* ignore */ }
                _xray = null;
            }
            return false;
        }
        finally
        {
            _gate.Release();
        }
    }

    /// <summary>Network handoff: re-pin the node route through the new gateway.</summary>
    private void OnNetworkChanged()
    {
        _ = Task.Run(async () =>
        {
            NetworkConfigurator? net;
            lock (_stateLock)
            {
                if (_state != TunnelState.Connected) return;
                net = _net;
            }
            if (net is null) return;

            _logger.LogInformation("network changed → re-pinning node route");
            var ok = await net.RepinNodeAsync(CancellationToken.None);
            if (!ok) _ = DegradeAsync("NETWORK CHANGED");
        });
    }

    /// <summary>
    /// Tears the tunnel down and reports Error. Serialized through the gate and a
    /// no-op if the user already disconnected or a reconnect is in flight.
    /// </summary>
    private async Task DegradeAsync(string reason)
    {
        await _gate.WaitAsync();
        try
        {
            lock (_stateLock) { if (_state != TunnelState.Connected) return; }
            _logger.LogWarning("degrade: {Reason}", reason);
            await TeardownAsync(CancellationToken.None);
            Set(TunnelState.Error, null, reason);
        }
        finally
        {
            _gate.Release();
        }
    }

    /// <summary>Drops the OS DNS resolver cache (best-effort; ignores failures).</summary>
    private void FlushDnsCache()
    {
        try
        {
            using var proc = System.Diagnostics.Process.Start(new System.Diagnostics.ProcessStartInfo
            {
                FileName = "ipconfig",
                Arguments = "/flushdns",
                UseShellExecute = false,
                CreateNoWindow = true,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
            });
            proc?.WaitForExit(3000);
        }
        catch (Exception ex)
        {
            _logger.LogDebug(ex, "flushdns");
        }
    }

    /// <summary>
    /// Forces the proxy outbound to establish immediately after connect by driving a
    /// few probes through the tunnel. Purely a warmup: results are NOT counted toward
    /// the degrade watchdog. Stops as soon as one succeeds.
    /// </summary>
    private async Task WarmUpAsync(CancellationToken token)
    {
        for (var i = 0; i < 6 && !token.IsCancellationRequested; i++)
        {
            lock (_stateLock) { if (_state != TunnelState.Connected) return; }
            var result = await TunnelHealth.ProbeAsync(token);
            if (result.Ok)
            {
                _logger.LogInformation("tunnel warmed up in ~{Ms}ms", result.ElapsedMs);
                return;
            }
            try { await Task.Delay(2000, token); }
            catch (OperationCanceledException) { return; }
        }
    }

    private void PushStats(long tx, long rx)
    {
        IpcStatus status;
        lock (_stateLock)
        {
            if (_state != TunnelState.Connected) return;
            _txBytes = tx;
            _rxBytes = rx;
            status = new IpcStatus { State = _state, Codename = _codename, TxBytes = tx, RxBytes = rx };
        }
        StatusChanged?.Invoke(status);
    }

    /// <summary>Reverses everything in dependency order. Individually fault-tolerant.</summary>
    private async Task TeardownAsync(CancellationToken ct)
    {
        StopMonitors();
        lock (_stateLock) { _txBytes = 0; _rxBytes = 0; }

        TunnelHealth.Reset();

        if (_net is not null)
        {
            try { await _net.RevertAsync(ct); } catch (Exception ex) { _logger.LogWarning(ex, "route revert"); }
            _net = null;
        }
        if (_xray is not null)
        {
            await _xray.DisposeAsync();
            _xray = null;
        }
        _adaptedConfigJson = null;
    }

    private IpcStatus Set(TunnelState state, string? codename, string? message)
    {
        IpcStatus status;
        lock (_stateLock)
        {
            _state = state;
            _codename = codename;
            _message = message;
            status = new IpcStatus { State = state, Codename = codename, Message = message };
        }
        StatusChanged?.Invoke(status);
        return status;
    }

    public async ValueTask DisposeAsync()
    {
        await TeardownAsync(CancellationToken.None);
        _gate.Dispose();
    }
}
