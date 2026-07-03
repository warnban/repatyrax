using Tyrax.Core.Models;

namespace Tyrax.Core.Abstractions;

/// <summary>
/// Auth surface mirroring the backend <c>/auth/*</c> routes. Implementations
/// throw <see cref="TyraxException"/> with an on-brand message on failure.
/// </summary>
public interface IAuthRepository
{
    Task<AuthResult> RegisterAsync(string email, string password, CancellationToken ct = default);
    Task<AuthResult> LoginAsync(string email, string password, CancellationToken ct = default);

    /// <summary>Confirms the 6-digit code emailed on registration; returns a session.</summary>
    Task<AuthResult> VerifyEmailAsync(string email, string code, CancellationToken ct = default);

    /// <summary>Requests a fresh confirmation code. Never throws for unknown emails.</summary>
    Task<bool> ResendVerificationAsync(string email, CancellationToken ct = default);

    /// <summary>Starts the Telegram deep-link flow; returns the bot URL + poll token.</summary>
    Task<TelegramInit> TelegramInitAsync(CancellationToken ct = default);

    /// <summary>Polls until the bot confirms the user; returns the auth result once ready.</summary>
    Task<AuthResult?> TelegramStatusAsync(string initToken, CancellationToken ct = default);

    Task<Profile> GetProfileAsync(CancellationToken ct = default);
}
