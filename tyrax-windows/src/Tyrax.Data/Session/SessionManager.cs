using Tyrax.Core.Abstractions;

namespace Tyrax.Data.Session;

/// <summary>
/// <see cref="ISession"/> over the <see cref="ISecureStore"/>. Holds the JWT and a
/// stable per-machine device name (<c>WIN-&lt;hostname&gt;</c>) generated once and
/// reused so the backend treats the PC as a single device across reinstalls.
/// </summary>
public sealed class SessionManager : ISession
{
    private const string TokenKey = "jwt";
    private const string UserIdKey = "user_id";
    private const string DeviceNameKey = "device_name";

    private readonly ISecureStore _store;

    public SessionManager(ISecureStore store)
    {
        _store = store;
        DeviceName = ResolveDeviceName();
    }

    public string? Token => _store.Get(TokenKey);

    public bool IsAuthenticated => !string.IsNullOrEmpty(Token);

    public string DeviceName { get; }

    public void SignIn(string token, string? userId)
    {
        _store.Set(TokenKey, token);
        if (!string.IsNullOrEmpty(userId)) _store.Set(UserIdKey, userId);
    }

    public void SignOut()
    {
        _store.Remove(TokenKey);
        _store.Remove(UserIdKey);
    }

    private string ResolveDeviceName()
    {
        var existing = _store.Get(DeviceNameKey);
        if (!string.IsNullOrEmpty(existing)) return existing;

        var host = Environment.MachineName;
        if (string.IsNullOrWhiteSpace(host)) host = "PC";
        var name = $"WIN-{host}";
        _store.Set(DeviceNameKey, name);
        return name;
    }
}
