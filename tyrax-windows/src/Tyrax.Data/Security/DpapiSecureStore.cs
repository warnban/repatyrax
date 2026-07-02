using System.IO;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using Tyrax.Core.Abstractions;

namespace Tyrax.Data.Security;

/// <summary>
/// <see cref="ISecureStore"/> backed by Windows DPAPI (CurrentUser scope). Values
/// are encrypted so only the signed-in Windows user can read them, and persisted
/// as a single blob under <c>%LOCALAPPDATA%\TYRAX\identity.dat</c>.
/// </summary>
public sealed class DpapiSecureStore : ISecureStore
{
    private static readonly byte[] Entropy = Encoding.UTF8.GetBytes("TYRAX.PROTOCOL.v1");
    private readonly string _path;
    private readonly object _gate = new();
    private Dictionary<string, string> _cache;

    public DpapiSecureStore()
    {
        var dir = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "TYRAX");
        Directory.CreateDirectory(dir);
        _path = Path.Combine(dir, "identity.dat");
        _cache = Load();
    }

    public string? Get(string key)
    {
        lock (_gate) return _cache.TryGetValue(key, out var v) ? v : null;
    }

    public void Set(string key, string value)
    {
        lock (_gate)
        {
            _cache[key] = value;
            Save();
        }
    }

    public void Remove(string key)
    {
        lock (_gate)
        {
            if (_cache.Remove(key)) Save();
        }
    }

    private Dictionary<string, string> Load()
    {
        if (!File.Exists(_path)) return new Dictionary<string, string>();
        try
        {
            var protectedBytes = File.ReadAllBytes(_path);
            var plain = ProtectedData.Unprotect(protectedBytes, Entropy, DataProtectionScope.CurrentUser);
            return JsonSerializer.Deserialize<Dictionary<string, string>>(plain)
                   ?? new Dictionary<string, string>();
        }
        catch (Exception)
        {
            // Corrupt or unreadable (e.g. copied from another user) — start clean.
            return new Dictionary<string, string>();
        }
    }

    private void Save()
    {
        var plain = JsonSerializer.SerializeToUtf8Bytes(_cache);
        var protectedBytes = ProtectedData.Protect(plain, Entropy, DataProtectionScope.CurrentUser);
        File.WriteAllBytes(_path, protectedBytes);
    }
}
