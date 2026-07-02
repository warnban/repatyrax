namespace Tyrax.Core.Models;

/// <summary>
/// Lifecycle of the PROTOCOL, driven by the privileged service and surfaced in the UI.
/// Copy stays on brand: OUTSIDE SYSTEM / BREACHING NETWORK… / ACCESS GRANTED.
/// </summary>
public enum TunnelState
{
    /// <summary>STATUS: OUTSIDE SYSTEM.</summary>
    Disconnected = 0,

    /// <summary>BREACHING NETWORK… (spawning xray + tun2socks, wiring routes).</summary>
    Connecting = 1,

    /// <summary>STATUS: ACCESS GRANTED.</summary>
    Connected = 2,

    /// <summary>Tearing routes down, stopping engines.</summary>
    Disconnecting = 3,

    /// <summary>CONNECTION FAILED. NODE UNAVAILABLE.</summary>
    Error = 4,
}
