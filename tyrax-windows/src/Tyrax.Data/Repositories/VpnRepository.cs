using Tyrax.Core;
using Tyrax.Core.Abstractions;
using Tyrax.Core.Models;
using Tyrax.Data.Remote;

namespace Tyrax.Data.Repositories;

/// <summary>Implements <see cref="IVpnRepository"/> over the Refit API.</summary>
public sealed class VpnRepository : IVpnRepository
{
    private const string OpenStatus = "OPEN";
    private readonly ITyraxApi _api;

    public VpnRepository(ITyraxApi api) => _api = api;

    public async Task<IReadOnlyList<Node>> GetNodesAsync(CancellationToken ct = default)
    {
        var list = await ApiErrors.UnwrapAsync(() => _api.GetNodesAsync(ct), "NODE UNAVAILABLE");
        return list.ConvertAll(ToDomain);
    }

    public async Task<Node> GetBestNodeAsync(CancellationToken ct = default)
    {
        var nodes = await GetNodesAsync(ct);
        // Server returns nodes already ordered by live load (least-loaded first),
        // so the first OPEN node is the balanced pick.
        foreach (var n in nodes)
            if (string.Equals(n.Status, OpenStatus, StringComparison.OrdinalIgnoreCase))
                return n;
        throw new TyraxException("NODE UNAVAILABLE");
    }

    public Task RegisterDeviceAsync(string name, CancellationToken ct = default)
        => ApiErrors.UnwrapAsync<object>(async () =>
        {
            var env = await _api.AddDeviceAsync(new AddDeviceRequest(name), ct);
            // AddDevice returns a structured device config; we only need success here.
            return new ApiEnvelope<object> { Status = env.Status, Data = env.Data ?? new object(), Message = env.Message };
        }, "DEVICE LIMIT REACHED");

    public async Task<IReadOnlyList<UserDevice>> GetDevicesAsync(CancellationToken ct = default)
    {
        var list = await ApiErrors.UnwrapAsync(() => _api.GetDevicesAsync(ct), "DEVICE LIST UNAVAILABLE");
        return list.ConvertAll(d => new UserDevice { Id = d.Id, Name = d.Name, CreatedAt = d.CreatedAt });
    }

    public async Task DeleteDeviceAsync(string deviceId, CancellationToken ct = default)
    {
        try { await _api.DeleteDeviceAsync(deviceId, ct); }
        catch (Exception e) { throw ApiErrors.Map(e); }
    }

    public async Task<ConnectConfig> ConnectAsync(string deviceName, string codename, CancellationToken ct = default)
    {
        var d = await ApiErrors.UnwrapAsync(
            () => _api.ConnectAsync(new VpnConnectRequest(deviceName, codename), ct), "NODE UNAVAILABLE");
        return new ConnectConfig { Protocol = d.Protocol, Config = d.Config };
    }

    public async Task<IReadOnlyList<string>> GetSplitDomainsAsync(CancellationToken ct = default)
    {
        try
        {
            var env = await _api.GetSplitDomainsAsync(ct);
            if (env.IsOk && env.Data is { Domains.Count: > 0 }) return env.Data.Domains;
        }
        catch (Exception) { /* fall through to defaults */ }
        return SplitTunnelDefaults.RuDomains;
    }

    public async Task<Subscription> GetSubscriptionAsync(CancellationToken ct = default)
    {
        var d = await ApiErrors.UnwrapAsync(() => _api.GetSubscriptionAsync(ct), "SUBSCRIPTION UNAVAILABLE");
        return new Subscription
        {
            Tier = d.Tier,
            EndsAt = d.EndsAt,
            DevicesCount = d.DevicesCount,
            DevicesLimit = d.DevicesLimit,
            TrafficUsedBytes = d.TrafficUsedBytes,
            TrafficLimitBytes = d.TrafficLimitBytes,
            Unlimited = d.Unlimited,
            BlockedUntil = d.BlockedUntil,
        };
    }

    private static Node ToDomain(NodeDto d) => new()
    {
        Id = d.Id,
        Codename = d.Codename,
        Country = d.Country,
        Status = d.Status,
        PingMs = d.PingMs,
        Load = d.Load,
        RealitySni = d.RealitySni,
    };
}
