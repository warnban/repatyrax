using Tyrax.Core.Models;

namespace Tyrax.Core.Abstractions;

/// <summary>
/// Subscription / payment / invite surface mirroring the backend
/// <c>/subscription*</c> and <c>/payment/*</c> routes. Implementations throw
/// <see cref="TyraxException"/> with an on-brand message on failure.
/// </summary>
public interface IBillingRepository
{
    Task<Subscription> GetSubscriptionAsync(CancellationToken ct = default);

    /// <summary>Creates an order and returns the hosted payment URL to open.</summary>
    Task<PaymentResult> CreatePaymentAsync(string tier, string method, int months, string email, CancellationToken ct = default);

    /// <summary>Polls one order; <see cref="PaymentStatus.OrderStatus"/> becomes <c>PAID</c> on success.</summary>
    Task<PaymentStatus> GetPaymentStatusAsync(string orderId, CancellationToken ct = default);

    // ── DOMINION invites ───────────────────────────────────────────────────────
    Task<IReadOnlyList<Invite>> GetInvitesAsync(CancellationToken ct = default);
    Task SendInviteAsync(string accountId, CancellationToken ct = default);
    Task RemoveInviteAsync(string accountId, CancellationToken ct = default);
}
