using System.Net.Http.Headers;
using Tyrax.Core.Abstractions;

namespace Tyrax.Data.Remote;

/// <summary>
/// Attaches <c>Authorization: Bearer &lt;jwt&gt;</c> to every request except the
/// public auth endpoints (adding one there would cause an auth loop). Mirrors the
/// Android <c>AuthInterceptor</c>.
/// </summary>
public sealed class AuthHeaderHandler : DelegatingHandler
{
    private static readonly string[] NoAuthPaths = { "/auth/register", "/auth/login" };
    private readonly ISession _session;

    public AuthHeaderHandler(ISession session) => _session = session;

    protected override Task<HttpResponseMessage> SendAsync(
        HttpRequestMessage request, CancellationToken cancellationToken)
    {
        var path = request.RequestUri?.AbsolutePath ?? "";
        var needsAuth = !Array.Exists(NoAuthPaths, p => path.EndsWith(p, StringComparison.OrdinalIgnoreCase));

        if (needsAuth && !string.IsNullOrEmpty(_session.Token))
            request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", _session.Token);

        return base.SendAsync(request, cancellationToken);
    }
}
