namespace Tyrax.Core.Abstractions;

/// <summary>
/// Encrypted-at-rest key/value store for secrets (JWT, device identity).
/// Implemented with Windows DPAPI (CurrentUser scope) in Tyrax.Data.
/// </summary>
public interface ISecureStore
{
    string? Get(string key);
    void Set(string key, string value);
    void Remove(string key);
}
