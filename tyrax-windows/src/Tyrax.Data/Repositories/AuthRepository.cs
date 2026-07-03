using System.Net;
using Refit;
using Tyrax.Core;
using Tyrax.Core.Abstractions;
using Tyrax.Core.Models;
using Tyrax.Data.Remote;

namespace Tyrax.Data.Repositories;

/// <summary>Implements <see cref="IAuthRepository"/> over the Refit API.</summary>
public sealed class AuthRepository : IAuthRepository
{
    private readonly ITyraxApi _api;

    public AuthRepository(ITyraxApi api) => _api = api;

    public async Task<AuthResult> RegisterAsync(string email, string password, CancellationToken ct = default)
    {
        var d = await ApiErrors.UnwrapAsync(() => _api.RegisterAsync(new RegisterRequest(email, password), ct), "REGISTRATION FAILED");
        return new AuthResult { Token = d.Token, UserId = d.UserId, VerificationRequired = d.VerificationRequired };
    }

    public async Task<AuthResult> LoginAsync(string email, string password, CancellationToken ct = default)
    {
        try
        {
            var env = await _api.LoginAsync(new LoginRequest(email, password), ct);
            if (env.IsOk && env.Data is not null)
                return new AuthResult { Token = env.Data.Token, UserId = env.Data.UserId };
            throw new TyraxException(env.Message ?? "INVALID CREDENTIALS");
        }
        // An unconfirmed identity is rejected with 403 + verification_required; the
        // backend has already re-sent a code, so route to the verify screen.
        catch (ApiException ex) when (ex.StatusCode == HttpStatusCode.Forbidden
                                      && (ex.Content?.Contains("verification_required") ?? false))
        {
            return new AuthResult { Token = "", UserId = null, VerificationRequired = true };
        }
        catch (Exception e)
        {
            throw ApiErrors.Map(e);
        }
    }

    public async Task<AuthResult> VerifyEmailAsync(string email, string code, CancellationToken ct = default)
    {
        var d = await ApiErrors.UnwrapAsync(() => _api.VerifyEmailAsync(new VerifyRequest(email, code), ct), "INVALID OR EXPIRED CODE");
        return new AuthResult { Token = d.Token, UserId = d.UserId };
    }

    public async Task ResendVerificationAsync(string email, CancellationToken ct = default)
    {
        try { await _api.ResendVerificationAsync(new ResendRequest(email), ct); }
        catch { /* Silent: the screen shows its own "code re-sent" hint regardless. */ }
    }

    public async Task<TelegramInit> TelegramInitAsync(CancellationToken ct = default)
    {
        var d = await ApiErrors.UnwrapAsync(() => _api.TelegramInitAsync(ct), "TELEGRAM UNAVAILABLE");
        return new TelegramInit { BotUrl = d.BotUrl, Token = d.Token };
    }

    public async Task<AuthResult?> TelegramStatusAsync(string initToken, CancellationToken ct = default)
    {
        try
        {
            var env = await _api.TelegramStatusAsync(initToken, ct);
            if (env.IsOk && env.Data is not null && !string.IsNullOrEmpty(env.Data.Token))
                return new AuthResult { Token = env.Data.Token, UserId = env.Data.UserId };
            return null; // not confirmed yet — caller keeps polling
        }
        catch (Exception)
        {
            return null;
        }
    }

    public async Task<Profile> GetProfileAsync(CancellationToken ct = default)
    {
        var d = await ApiErrors.UnwrapAsync(() => _api.GetProfileAsync(ct), "IDENTITY NOT FOUND");
        return new Profile
        {
            UserId = d.UserId,
            Email = d.Email,
            Tier = d.Tier,
            TelegramLinked = d.TelegramLinked,
        };
    }
}
