using CommunityToolkit.Mvvm.Input;
using Tyrax.Core.Models;

namespace Tyrax.App.ViewModels;

/// <summary>One tier row on the CONTROL (subscription) screen.</summary>
public sealed class TierCardViewModel
{
    private readonly Action<string> _onUnlock;

    public TierCardViewModel(string name, bool isCurrent, Action<string> onUnlock)
    {
        Name = name;
        IsCurrent = isCurrent;
        _onUnlock = onUnlock;
        UnlockCommand = new RelayCommand(() => _onUnlock(Name));
    }

    public string Name { get; }
    public bool IsCurrent { get; }
    public string Features => BillingPlan.Features(Name);
    public string PriceText => BillingPlan.PriceLine(Name);

    /// <summary>UNLOCK is offered only for paid tiers the user isn't already on.</summary>
    public bool CanUnlock => !IsCurrent && BillingPlan.BasePrice(Name) > 0;

    public IRelayCommand UnlockCommand { get; }
}
