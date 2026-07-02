namespace Tyrax.Core.Models;

/// <summary>Тарифы и цены для UI (бэкенд — источник истины при оплате).</summary>
public static class BillingPlan
{
    public static readonly IReadOnlyList<string> Tiers = new[] { "CORE", "SHADOW", "DOMINION" };
    public static readonly IReadOnlyList<string> DisplayTiers = new[] { "FREE", "CORE", "SHADOW", "DOMINION" };
    public static readonly IReadOnlyList<int> Months = new[] { 1, 3, 6, 12 };

    private static readonly Dictionary<string, int> BasePrices = new(StringComparer.OrdinalIgnoreCase)
    {
        ["CORE"] = 199,
        ["SHADOW"] = 349,
        ["DOMINION"] = 649,
    };

    private static readonly Dictionary<int, double> Discounts = new()
    {
        [1] = 1.00,
        [3] = 0.90,
        [6] = 0.85,
        [12] = 0.80,
    };

    public static int BasePrice(string tier) => BasePrices.TryGetValue(tier, out var p) ? p : 0;

    public static (int Total, int Monthly, int Saving) Quote(string tier, int months)
    {
        var basePrice = BasePrice(tier);
        var discount = Discounts.TryGetValue(months, out var d) ? d : 1.0;
        var total = (int)Math.Round(basePrice * months * discount, MidpointRounding.AwayFromZero);
        var monthly = months > 0 ? (int)Math.Round((double)total / months, MidpointRounding.AwayFromZero) : total;
        var saving = basePrice * months - total;
        return (total, monthly, saving);
    }

    public static string Features(string tier) => tier.ToUpperInvariant() switch
    {
        "FREE" => "1 УСТРОЙСТВО · ЛИМИТ ТРАФИКА · БАЗОВЫЕ ЛОКАЦИИ",
        "CORE" => "2 УСТРОЙСТВА · БЕЗЛИМИТ · ВСЕ ЛОКАЦИИ",
        "SHADOW" => "5 УСТРОЙСТВ · УСКОРЕНИЕ · БЕЗЛИМИТ · ВСЕ ЛОКАЦИИ",
        "DOMINION" => "10 УСТРОЙСТВ · УСКОРЕНИЕ · БЕЗЛИМИТ · ПРИОРИТЕТ · СУБ-АККАУНТЫ",
        _ => "",
    };

    public static string PriceLine(string tier)
    {
        var price = BasePrice(tier);
        return price <= 0 ? "БАЗОВЫЙ · 0 ₽" : $"ОТ {price} ₽/МЕС";
    }
}
