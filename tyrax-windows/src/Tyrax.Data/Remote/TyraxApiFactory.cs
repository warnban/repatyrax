using Refit;
using Tyrax.Core.Abstractions;

namespace Tyrax.Data.Remote;

/// <summary>
/// Builds the Refit <see cref="ITyraxApi"/> over an <see cref="HttpClient"/> whose
/// pipeline attaches the Bearer token. Base URL matches the Android client.
/// </summary>
public static class TyraxApiFactory
{
    // Production. For local dev against a machine-local backend, override via Create.
    public const string DefaultBaseUrl = "https://api.tyrax.tech/api/v1";

    public static ITyraxApi Create(ISession session, string? baseUrl = null)
    {
        var handler = new AuthHeaderHandler(session) { InnerHandler = new HttpClientHandler() };
        var http = new HttpClient(handler)
        {
            BaseAddress = new Uri(baseUrl ?? DefaultBaseUrl),
            Timeout = TimeSpan.FromSeconds(30),
        };
        return RestService.For<ITyraxApi>(http);
    }
}
