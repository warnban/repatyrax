using System.Linq;
using Tyrax.Core;
using Tyrax.Core.Abstractions;
using Tyrax.Core.Models;
using Tyrax.Ipc;

namespace Tyrax.App.Services;

/// <summary>
/// Keeps the tunnel alive "in any conditions". Picks the best node, tells the
/// service to breach it, then watches the IPC status stream; when the service
/// reports the tunnel dropped or degraded (<see cref="TunnelState.Error"/>), it
/// silently switches to the next candidate node. Direct port of the Android
/// <c>ConnectionSupervisor</c>, but node health is measured service-side and
/// surfaced over the pipe.
/// </summary>
public sealed class ConnectionSupervisor : IDisposable
{
    // Service-side connect can exceed 25s (xray TUN wait + netsh retries on WinTun).
    private static readonly TimeSpan ConnectTimeout = TimeSpan.FromSeconds(60);
    private static readonly TimeSpan SwitchDelay = TimeSpan.FromMilliseconds(1200);
    private static readonly TimeSpan Backoff = TimeSpan.FromSeconds(8);

    private readonly IVpnRepository _vpn;
    private readonly ISession _session;
    private readonly TunnelIpcClient _ipc;

    private readonly object _gate = new();
    private volatile bool _wants;
    private CancellationTokenSource? _loopCts;
    private Task _loopTask = Task.CompletedTask;
    private CancellationTokenSource _dropCts = new();

    public ConnectionSupervisor(IVpnRepository vpn, ISession session, TunnelIpcClient ipc)
    {
        _vpn = vpn;
        _session = session;
        _ipc = ipc;
        _ipc.Disconnected += OnPipeDrop;
    }

    /// <summary>User pressed ENTER, or chose a node. Restarts supervision, preferring <paramref name="preferredCodename"/>.</summary>
    public void Start(string? preferredCodename = null) => _ = RestartAsync(preferredCodename);

    /// <summary>User pressed DISCONNECT. Stops supervision and tears the tunnel down.</summary>
    public async Task StopAsync()
    {
        _wants = false;
        await StopLoopAsync();
        try { await _ipc.SendAsync(new IpcCommand { Kind = IpcCommandKind.Disconnect }); }
        catch (Exception) { /* pipe may be down */ }
    }

    private async Task RestartAsync(string? preferred)
    {
        await StopLoopAsync();
        _wants = true;
        var cts = new CancellationTokenSource();
        lock (_gate) _loopCts = cts;
        _loopTask = Task.Run(() => RunAsync(preferred, cts.Token));
    }

    private async Task StopLoopAsync()
    {
        CancellationTokenSource? cts;
        Task task;
        lock (_gate) { cts = _loopCts; task = _loopTask; _loopCts = null; }
        cts?.Cancel();
        try { await task; } catch (Exception) { /* ignore */ }
    }

    private async Task RunAsync(string? preferred, CancellationToken ct)
    {
        var candidates = await LoadCandidatesAsync(ct);
        if (candidates.Count == 0) return; // MainViewModel surfaces NODE UNAVAILABLE via IPC error

        if (preferred is not null)
        {
            var i = candidates.FindIndex(n => string.Equals(n.Codename, preferred, StringComparison.OrdinalIgnoreCase));
            if (i > 0) { var n = candidates[i]; candidates.RemoveAt(i); candidates.Insert(0, n); }
        }

        var idx = 0;
        var fails = 0;

        while (_wants && !ct.IsCancellationRequested)
        {
            var node = candidates[idx % candidates.Count];
            var connected = await AttemptNodeAsync(node, ct);

            if (!connected)
            {
                if (!_wants) break;
                fails++;
                idx++;
                if (fails >= candidates.Count * 2)
                {
                    await DelayQuiet(Backoff, ct);
                    fails = 0;
                    var refreshed = await LoadCandidatesAsync(ct);
                    if (refreshed.Count > 0) candidates = refreshed;
                }
                continue;
            }

            fails = 0;
            await MonitorUntilDropAsync(ct);
            if (!_wants) break;

            await DelayQuiet(SwitchDelay, ct);
            idx++;
        }
    }

    private async Task<bool> AttemptNodeAsync(Node node, CancellationToken ct)
    {
        try
        {
            var config = await _vpn.ConnectAsync(_session.DeviceName, node.Codename, ct);
            var split = await _vpn.GetSplitDomainsAsync(ct);
            await _ipc.SendAsync(new IpcCommand
            {
                Kind = IpcCommandKind.Connect,
                Codename = node.Codename,
                Protocol = config.Protocol,
                ConfigJson = config.Config,
                SplitDomains = (IReadOnlyList<string>)split,
            }, ct);
        }
        catch (TyraxException)
        {
            return false;
        }
        catch (Exception)
        {
            return false; // pipe send failed etc.
        }

        var reached = await WaitForStatusAsync(s => s.State is TunnelState.Connected or TunnelState.Error, ConnectTimeout, ct);
        return reached?.State == TunnelState.Connected;
    }

    private async Task MonitorUntilDropAsync(CancellationToken ct)
        => await WaitForStatusAsync(s => s.State != TunnelState.Connected, timeout: null, ct);

    /// <summary>
    /// Completes with the first status matching <paramref name="predicate"/>, or
    /// null on timeout / pipe drop / cancellation.
    /// </summary>
    private async Task<IpcStatus?> WaitForStatusAsync(Func<IpcStatus, bool> predicate, TimeSpan? timeout, CancellationToken ct)
    {
        var tcs = new TaskCompletionSource<IpcStatus>(TaskCreationOptions.RunContinuationsAsynchronously);
        void Handler(IpcStatus s) { if (predicate(s)) tcs.TrySetResult(s); }

        _ipc.StatusReceived += Handler;
        using var linked = CancellationTokenSource.CreateLinkedTokenSource(ct, _dropCts.Token);
        if (timeout is { } t) linked.CancelAfter(t);
        using var reg = linked.Token.Register(() => tcs.TrySetCanceled());
        try
        {
            return await tcs.Task;
        }
        catch (OperationCanceledException)
        {
            return null;
        }
        finally
        {
            _ipc.StatusReceived -= Handler;
        }
    }

    private async Task<List<Node>> LoadCandidatesAsync(CancellationToken ct)
    {
        try
        {
            var nodes = await _vpn.GetNodesAsync(ct);
            var open = nodes.Where(n => string.Equals(n.Status, "OPEN", StringComparison.OrdinalIgnoreCase)).ToList();
            if (open.Count > 0) return open;

            var best = await _vpn.GetBestNodeAsync(ct);
            return new List<Node> { best };
        }
        catch (Exception)
        {
            return new List<Node>();
        }
    }

    private void OnPipeDrop()
    {
        // Unblock any in-flight status wait; the loop treats it as a drop.
        var old = _dropCts;
        _dropCts = new CancellationTokenSource();
        old.Cancel();
        old.Dispose();
    }

    private static async Task DelayQuiet(TimeSpan d, CancellationToken ct)
    {
        try { await Task.Delay(d, ct); } catch (OperationCanceledException) { }
    }

    public void Dispose()
    {
        _ipc.Disconnected -= OnPipeDrop;
        _loopCts?.Cancel();
        _dropCts.Dispose();
    }
}
