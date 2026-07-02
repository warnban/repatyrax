namespace Tyrax.Core.Models;

/// <summary>
/// Access levels. Device limits match the backend <c>DeviceLimit</c>: 1 / 2 / 5 / 10.
/// </summary>
public enum SubscriptionTier
{
    Free = 0,
    Core = 1,
    Shadow = 2,
    Dominion = 3,
}

public static class SubscriptionTierExtensions
{
    /// <summary>Max devices per account for the tier — mirrors backend <c>DeviceLimit</c>.</summary>
    public static int DeviceLimit(this SubscriptionTier tier) => tier switch
    {
        SubscriptionTier.Free => 1,
        SubscriptionTier.Core => 2,
        SubscriptionTier.Shadow => 5,
        SubscriptionTier.Dominion => 10,
        _ => 1,
    };
}
