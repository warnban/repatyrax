using System.Net.NetworkInformation;

namespace Tyrax.Service.Net;

/// <summary>
/// Reads cumulative byte counters straight off the WinTun adapter. Because the
/// adapter is created fresh on each connect, the counters equal the session total
/// — no baseline bookkeeping needed. Counts exactly what crosses the tunnel.
/// </summary>
public static class NetworkStats
{
    /// <summary>
    /// Reads bytes sent (tx / upload) and received (rx / download) for the named
    /// adapter. Returns false if the adapter is not present or has no IPv4 stats.
    /// </summary>
    public static bool TryRead(string adapterName, out long tx, out long rx)
    {
        tx = 0;
        rx = 0;
        foreach (var nic in NetworkInterface.GetAllNetworkInterfaces())
        {
            if (!string.Equals(nic.Name, adapterName, StringComparison.OrdinalIgnoreCase)) continue;
            try
            {
                var s = nic.GetIPv4Statistics();
                tx = s.BytesSent;
                rx = s.BytesReceived;
                return true;
            }
            catch (NetworkInformationException)
            {
                return false;
            }
        }
        return false;
    }
}
