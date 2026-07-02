using Tyrax.Core.Models;

namespace Tyrax.Core.Abstractions;

/// <summary>
/// Node / device / tunnel surface mirroring the backend <c>/vpn/*</c>,
/// <c>/nodes</c> and <c>/subscription</c> routes.
/// </summary>
public interface IVpnRepository
{
    Task<IReadOnlyList<Node>> GetNodesAsync(CancellationToken ct = default);

    /// <summary>First OPEN node in the backend's load-balanced order.</summary>
    Task<Node> GetBestNodeAsync(CancellationToken ct = default);

    /// <summary>Registers this device (idempotent by name) on the account.</summary>
    Task RegisterDeviceAsync(string name, CancellationToken ct = default);

    Task<IReadOnlyList<UserDevice>> GetDevicesAsync(CancellationToken ct = default);
    Task DeleteDeviceAsync(string deviceId, CancellationToken ct = default);

    /// <summary>Fetches a ready-to-run tunnel config for the device on the given node.</summary>
    Task<ConnectConfig> ConnectAsync(string deviceName, string codename, CancellationToken ct = default);

    /// <summary>RU domains that must bypass the tunnel. Falls back to a built-in list.</summary>
    Task<IReadOnlyList<string>> GetSplitDomainsAsync(CancellationToken ct = default);

    Task<Subscription> GetSubscriptionAsync(CancellationToken ct = default);
}
