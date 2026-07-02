using Tyrax.Core;
using Tyrax.Core.Abstractions;
using Tyrax.Core.Models;
using Tyrax.Data.Remote;

namespace Tyrax.Data.Repositories;

/// <summary>Implements <see cref="IBillingRepository"/> over the Refit API.</summary>
public sealed class BillingRepository : IBillingRepository
{
    private readonly ITyraxApi _api;

    public BillingRepository(ITyraxApi api) => _api = api;

    public async Task<Subscription> GetSubscriptionAsync(CancellationToken ct = default)
    {
        var d = await ApiErrors.UnwrapAsync(() => _api.GetSubscriptionAsync(ct), "SUBSCRIPTION UNAVAILABLE");
        return new Subscription
        {
            Tier = d.Tier,
            EndsAt = d.EndsAt,
            DevicesCount = d.DevicesCount,
            DevicesLimit = d.DevicesLimit,
            TrafficUsedBytes = d.TrafficUsedBytes,
            TrafficLimitBytes = d.TrafficLimitBytes,
            Unlimited = d.Unlimited,
            BlockedUntil = d.BlockedUntil,
        };
    }

    public async Task<PaymentResult> CreatePaymentAsync(string tier, string method, int months, string email, CancellationToken ct = default)
    {
        // ip left empty — the backend fills it from the request source address.
        var d = await ApiErrors.UnwrapAsync(
            () => _api.CreatePaymentAsync(new CreatePaymentRequest(tier, method, months, email, ""), ct),
            "PAYMENT CREATION FAILED");
        return new PaymentResult { OrderId = d.OrderId, PaymentUrl = d.PaymentUrl, AmountRub = d.AmountRub };
    }

    public async Task<PaymentStatus> GetPaymentStatusAsync(string orderId, CancellationToken ct = default)
    {
        var d = await ApiErrors.UnwrapAsync(() => _api.GetPaymentStatusAsync(orderId, ct), "ORDER NOT FOUND");
        return new PaymentStatus { OrderStatus = d.OrderStatus, Tier = d.Tier };
    }

    public async Task<IReadOnlyList<Invite>> GetInvitesAsync(CancellationToken ct = default)
    {
        var list = await ApiErrors.UnwrapAsync(() => _api.GetInvitesAsync(ct), "INVITES UNAVAILABLE");
        return list.ConvertAll(i => new Invite { Id = i.Id, InviteeId = i.InviteeId, Status = i.Status });
    }

    public async Task SendInviteAsync(string accountId, CancellationToken ct = default)
    {
        try
        {
            var env = await _api.SendInviteAsync(new SendInviteRequest(accountId), ct);
            if (!env.IsOk) throw new TyraxException(env.Message ?? "INVITE FAILED");
        }
        catch (TyraxException) { throw; }
        catch (Exception e) { throw ApiErrors.Map(e); }
    }

    public async Task RemoveInviteAsync(string accountId, CancellationToken ct = default)
    {
        try
        {
            var env = await _api.RemoveInviteAsync(accountId, ct);
            if (!env.IsOk) throw new TyraxException(env.Message ?? "REMOVE FAILED");
        }
        catch (TyraxException) { throw; }
        catch (Exception e) { throw ApiErrors.Map(e); }
    }
}
