namespace Tyrax.Core.Models;

/// <summary>Result of a successful auth exchange (register/login/telegram).</summary>
public sealed record AuthResult
{
    public required string Token { get; init; }
    public string? UserId { get; init; }

    /// <summary>
    /// True when the backend requires email confirmation before issuing a session
    /// (register response, or a login attempt by an unconfirmed identity). The
    /// <see cref="Token"/> is empty in that case — route to the verify screen.
    /// </summary>
    public bool VerificationRequired { get; init; }

    /// <summary>False when SMTP delivery failed after registration/resend.</summary>
    public bool EmailSent { get; init; } = true;
}

/// <summary>The signed-in IDENTITY as returned by <c>/auth/profile</c>.</summary>
public sealed record Profile
{
    public string? UserId { get; init; }
    public string? Email { get; init; }
    public string? Tier { get; init; }
    public bool TelegramLinked { get; init; }
}

/// <summary>Telegram deep-link handshake payload from <c>/auth/telegram-init</c>.</summary>
public sealed record TelegramInit
{
    public required string BotUrl { get; init; }
    public required string Token { get; init; }
}
