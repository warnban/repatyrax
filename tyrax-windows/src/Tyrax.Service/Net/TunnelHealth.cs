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
    // Several independent 204/tiny endpoints. A single blocked/throttled host must
    // never be read as a dead tunnel, so the probe passes if ANY of them answers.
    private static readonly string[] ProbeUrls =
    {
        "https://www.gstatic.com/generate_204",
        "https://cp.cloudflare.com/generate_204",
        "https://www.google.com/generate_204",
    };
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
        var http = GetOrCreateClient();

        foreach (var url in ProbeUrls)
        {
            try
            {
                using var resp = await http.GetAsync(url, HttpCompletionOption.ResponseHeadersRead, ct);
                if (resp.IsSuccessStatusCode || (int)resp.StatusCode == 204)
                    return new Result(true, sw.ElapsedMilliseconds);
            }
            catch (OperationCanceledException) when (ct.IsCancellationRequested)
            {
                return new Result(false, long.MaxValue);
            }
            catch (Exception)
            {
                // try the next endpoint before declaring the tunnel unhealthy
            }
        }

        return new Result(false, sw.ElapsedMilliseconds);
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
