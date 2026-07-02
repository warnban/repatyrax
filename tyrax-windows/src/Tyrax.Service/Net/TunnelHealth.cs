using System.Diagnostics;
using System.Net.Http;

namespace Tyrax.Service.Net;

/// <summary>
/// Liveness / throttle probe for the active tunnel. Probes through the system
/// default route (split-default via WinTun) so it exercises the full TUN → xray
/// → VLESS → node → internet path — the same stack user traffic hits.
/// </summary>
public static class TunnelHealth
{
    private const string ProbeUrl = "https://www.gstatic.com/generate_204";
    private static readonly TimeSpan Timeout = TimeSpan.FromSeconds(10);

    private static readonly object Gate = new();
    private static HttpClient? _client;
    private static SocketsHttpHandler? _handler;

    public readonly record struct Result(bool Ok, long ElapsedMs);

    /// <summary>Drops the pooled client (call on tunnel teardown).</summary>
    public static void Reset()
    {
        lock (Gate)
        {
            _client?.Dispose();
            _handler?.Dispose();
            _client = null;
            _handler = null;
        }
    }

    public static async Task<Result> ProbeAsync(CancellationToken ct)
    {
        var sw = Stopwatch.StartNew();
        try
        {
            var http = GetOrCreateClient();
            using var resp = await http.GetAsync(ProbeUrl, HttpCompletionOption.ResponseHeadersRead, ct);
            var ok = resp.IsSuccessStatusCode || (int)resp.StatusCode == 204;
            return new Result(ok, sw.ElapsedMilliseconds);
        }
        catch (Exception)
        {
            return new Result(false, long.MaxValue);
        }
    }

    private static HttpClient GetOrCreateClient()
    {
        lock (Gate)
        {
            if (_client is not null)
                return _client;

            _handler = new SocketsHttpHandler
            {
                PooledConnectionLifetime = TimeSpan.FromMinutes(10),
                ConnectTimeout = Timeout,
            };
            _client = new HttpClient(_handler, disposeHandler: false) { Timeout = Timeout };
            return _client;
        }
    }
}
