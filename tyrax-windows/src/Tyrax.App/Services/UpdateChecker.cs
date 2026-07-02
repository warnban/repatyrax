using System.Net.Http;
using System.Reflection;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace Tyrax.App.Services;

/// <summary>Newer build available on the download endpoint.</summary>
public sealed record UpdateInfo(Version Version, string Url);

/// <summary>
/// Checks the release manifest for a newer signed installer. The app is installed
/// machine-wide with a service (see installer\tyrax.iss), so updates ship as a new
/// signed installer rather than via a per-user updater like Velopack — this only
/// detects and points the user at it; it never downloads/runs anything silently.
/// Fail-silent: any network/parse error just reports "no update".
/// </summary>
public sealed class UpdateChecker
{
    private const string ManifestUrl = "https://api.tyrax.tech/download/windows/latest.json";
    private static readonly TimeSpan Timeout = TimeSpan.FromSeconds(8);

    public async Task<UpdateInfo?> CheckAsync(CancellationToken ct = default)
    {
        try
        {
            using var http = new HttpClient { Timeout = Timeout };
            var json = await http.GetStringAsync(ManifestUrl, ct);
            var manifest = JsonSerializer.Deserialize<Manifest>(json);
            if (manifest is null || string.IsNullOrWhiteSpace(manifest.Version) || string.IsNullOrWhiteSpace(manifest.Url))
                return null;
            if (!Version.TryParse(manifest.Version, out var latest)) return null;

            var current = Assembly.GetExecutingAssembly().GetName().Version ?? new Version(0, 0);
            return latest > current ? new UpdateInfo(latest, manifest.Url!) : null;
        }
        catch (Exception)
        {
            return null;
        }
    }

    private sealed class Manifest
    {
        [JsonPropertyName("version")] public string? Version { get; set; }
        [JsonPropertyName("url")] public string? Url { get; set; }
    }
}
