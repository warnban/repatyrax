namespace Tyrax.Core.Abstractions;

/// <summary>
/// Holds the current auth token and this machine's stable device name, backed by
/// the <see cref="ISecureStore"/>. The API layer reads <see cref="Token"/> to
/// attach the Bearer header.
/// </summary>
public interface ISession
{
    /// <summary>Current JWT, or <c>null</c> when signed out.</summary>
    string? Token { get; }

    bool IsAuthenticated { get; }

    /// <summary>Stable per-machine device name, e.g. <c>WIN-&lt;hostname&gt;</c>. Created once.</summary>
    string DeviceName { get; }

    void SignIn(string token, string? userId);
    void SignOut();
}
