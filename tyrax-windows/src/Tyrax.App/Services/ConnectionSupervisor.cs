using System.Linq;
using Tyrax.Core;
using Tyrax.Core.Abstractions;
using Tyrax.Core.Models;
using Tyrax.Ipc;

namespace Tyrax.App.Services;

/// <summary>Intent the supervisor is currently pursuing, surfaced to the UI so the
/// main screen can show a coherent status/button independent of transient tunnel
/// state pushes.</summary>
public enum SupervisorPhase
{
    /// <summary>User does not want a tunnel — button reads ВКЛЮЧИТЬ.</summary>
    Idle,

    /// <summary>User wants a tunnel; establishing or holding it — button reads ВЫКЛЮЧИТЬ.</summary>
    Working,

    /// <summary>User wants a tunnel; a drop/failure is being recovered — button reads ВЫКЛЮЧИТЬ.</summary>
    Reconnecting,
}

/// <summary>
/// Keeps the tunnel alive "in any conditions". Picks the best node, tells the
/// service to breach it, then watches the IPC status stream; when the service
/// reports the tunnel dropped or degraded (<see cref="TunnelState.Error"/>), it
/// silently switches to the next candidate node. Direct port of the Android
/// <c>ConnectionSupervisor</c>, but node health is measured service-side and
/// surfaced over the pipe.
///
/// <para>The UI must drive its button off <see cref="IsActive"/> / <see cref="PhaseChanged"/>
/// (the user's intent), NOT off the raw tunnel state — otherwise a transient
/// <see cref="TunnelState.Error"/> during an in-flight reconnect makes the button
/// flip to ВКЛЮЧИТЬ and the user can neither stop nor see the recovery.</para>
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
    private readonly SemaphoreSlim _transition = new(1, 1);
    private volatile bool _wants;
    private SupervisorPhase _phase = SupervisorPhase.Idle;
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

    /// <summary>True while the user wants a tunnel (establishing, online, or recovering).</summary>
    public bool IsActive => _wants;

    /// <summary>Current supervisor phase, so the UI shows a coherent status + button.</summary>
    public SupervisorPhase Phase => _phase;

    /// <summary>Raised whenever <see cref="Phase"/> changes.</summary>
    public event Action<SupervisorPhase>? PhaseChanged;

    /// <summary>User pressed ENTER, or chose a node. Restarts supervision, preferring <paramref name="preferredCodename"/>.</summary>
    public void Start(string? preferredCodename = null) => _ = RestartAsync(preferredCodename);

    /// <summary>User pressed DISCONNECT. Stops supervision and tears the tunnel down.</summary>
    public async Task StopAsync()
    {
        await _transition.WaitAsync();
        try
        {
            _wants = false;
            SetPhase(SupervisorPhase.Idle);
            await StopLoopAsync();
            try { await _ipc.SendAsync(new IpcCommand { Kind = IpcCommandKind.Disconnect }); }
            catch (Exception) { /* pipe may be down */ }
        }
        finally
        {
            _transition.Release();
        }
    }

    private async Task RestartAsync(string? preferred)
    {
        await _transition.WaitAsync();
        try
        {
            await StopLoopAsync();
            _wants = true;
            SetPhase(SupervisorPhase.Working);
            var cts = new CancellationTokenSource();
            lock (_gate) _loopCts = cts;
            _loopTask = Task.Run(() => RunAsync(preferred, cts.Token));
        }
        finally
        {
            _transition.Release();
        }
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
        List<Node> candidates = new();
        var idx = 0;
        var fails = 0;

        while (_wants && !ct.IsCancellationRequested)
        {
            // (Re)load candidates and self-heal when none are available yet.
            if (candidates.Count == 0)
            {
                candidates = await LoadCandidatesAsync(ct);
                if (candidates.Count == 0)
                {
                    SetPhase(SupervisorPhase.Reconnecting);
                    await DelayQuiet(Backoff, ct);
                    continue;
                }

                if (preferred is not null)
                {
                    var i = candidates.FindIndex(n => string.Equals(n.Codename, preferred, StringComparison.OrdinalIgnoreCase));
                    if (i > 0) { var n = candidates[i]; candidates.RemoveAt(i); candidates.Insert(0, n); }
                }
                idx = 0;
            }

            var node = candidates[idx % candidates.Count];
            var connected = await AttemptNodeAsync(node, ct);

            if (!connected)
            {
                if (!_wants) break;
                fails++;
                idx++;
                SetPhase(SupervisorPhase.Reconnecting);
                if (fails >= candidates.Count * 2)
                {
                    await DelayQuiet(Backoff, ct);
                    fails = 0;
                    candidates = new(); // force a fresh node list next iteration
                }
                continue;
            }

            fails = 0;
            SetPhase(SupervisorPhase.Working);
            await MonitorUntilDropAsync(ct);
            if (!_wants) break;

            // The tunnel dropped while the user still wants it → recover.
            SetPhase(SupervisorPhase.Reconnecting);
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

    private void SetPhase(SupervisorPhase phase)
    {
        bool changed;
        lock (_gate) { changed = _phase != phase; _phase = phase; }
        if (changed) PhaseChanged?.Invoke(phase);
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
        _transition.Dispose();
    }
}
