using System.Collections.ObjectModel;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Tyrax.Core;
using Tyrax.Core.Abstractions;
using Tyrax.Core.Models;

namespace Tyrax.App.ViewModels;

/// <summary>Экран «Тарифы»: план, расход трафика, апгрейд, инвайты DOMINION.</summary>
public sealed partial class SubscriptionViewModel : ObservableObject
{
    private readonly IBillingRepository _billing;

    [ObservableProperty] private bool _busy;
    [ObservableProperty] private string _error = "";
    [ObservableProperty] private string _currentTier = "";
    [ObservableProperty] private string _statusLine = "";
    [ObservableProperty] private bool _isDominion;
    [ObservableProperty] private string _newInviteId = "";
    [ObservableProperty] private string _inviteError = "";

    [ObservableProperty] private bool _showUsage;
    [ObservableProperty] private bool _isUnlimited;
    [ObservableProperty] private string _usageTitle = "";
    [ObservableProperty] private string _usageDetail = "";
    [ObservableProperty] private double _usagePercent;
    [ObservableProperty] private bool _showBlocked;
    [ObservableProperty] private string _blockedLine = "";

    public ObservableCollection<TierCardViewModel> Tiers { get; } = new();
    public ObservableCollection<InviteItemViewModel> Invites { get; } = new();

    public SubscriptionViewModel(IBillingRepository billing) => _billing = billing;

    public event Action? BackRequested;
    public event Action<string>? PayRequested;

    [RelayCommand]
    private void Back() => BackRequested?.Invoke();

    /// <summary>Синхронизирует блок расхода без полного рефреша (с главного экрана).</summary>
    public void ApplySubscription(Subscription sub) => OnUi(() => ApplyUsage(sub));

    [RelayCommand]
    public async Task RefreshAsync()
    {
        OnUi(() => { Busy = true; Error = ""; });
        try
        {
            var sub = await _billing.GetSubscriptionAsync();
            var tier = sub.Tier.ToUpperInvariant();

            OnUi(() =>
            {
                CurrentTier = tier;
                StatusLine = BuildStatusLine(sub);
                IsDominion = tier == "DOMINION";
                ApplyUsage(sub);

                Tiers.Clear();
                foreach (var t in BillingPlan.DisplayTiers)
                    Tiers.Add(new TierCardViewModel(t, t == tier, OnUnlock));
            });

            if (tier == "DOMINION") await LoadInvitesAsync();
            else OnUi(() => Invites.Clear());
        }
        catch (TyraxException ex)
        {
            OnUi(() => Error = ex.Message);
        }
        finally
        {
            OnUi(() => Busy = false);
        }
    }

    private void ApplyUsage(Subscription sub)
    {
        if (!string.IsNullOrEmpty(sub.BlockedUntil))
        {
            ShowBlocked = true;
            BlockedLine = $"ДОСТУП ИСЧЕРПАН · РАЗБЛОКИРОВКА {FormatDate(sub.BlockedUntil)}";
        }
        else
        {
            ShowBlocked = false;
            BlockedLine = "";
        }

        if (sub.Unlimited || sub.TrafficLimitBytes <= 0)
        {
            ShowUsage = sub.Unlimited;
            IsUnlimited = true;
            UsageTitle = "ТРАФИК";
            UsageDetail = "БЕЗЛИМИТ";
            UsagePercent = 0;
            return;
        }

        ShowUsage = true;
        IsUnlimited = false;
        var usedGb = sub.TrafficUsedBytes / 1024.0 / 1024.0 / 1024.0;
        var limitGb = sub.TrafficLimitBytes / 1024.0 / 1024.0 / 1024.0;
        var remaining = Math.Max(0, limitGb - usedGb);
        UsageTitle = "РАСХОД ТРАФИКА";
        UsageDetail = $"ИСПОЛЬЗОВАНО {usedGb:0.0} / {limitGb:0.0} ГБ · ОСТАЛОСЬ {remaining:0.0} ГБ";
        UsagePercent = limitGb > 0 ? Math.Min(100, usedGb / limitGb * 100) : 0;
    }

    private void OnUnlock(string tier) => PayRequested?.Invoke(tier);

    private async Task LoadInvitesAsync()
    {
        try
        {
            var invites = await _billing.GetInvitesAsync();
            OnUi(() =>
            {
                Invites.Clear();
                foreach (var i in invites)
                    Invites.Add(new InviteItemViewModel(i, id => _ = RemoveInviteAsync(id)));
            });
        }
        catch (TyraxException ex)
        {
            OnUi(() => InviteError = ex.Message);
        }
    }

    [RelayCommand]
    private async Task SendInviteAsync()
    {
        var id = NewInviteId.Trim();
        if (id.Length == 0) return;
        OnUi(() => InviteError = "");
        try
        {
            await _billing.SendInviteAsync(id);
            OnUi(() => NewInviteId = "");
            await LoadInvitesAsync();
        }
        catch (TyraxException ex)
        {
            OnUi(() => InviteError = ex.Message);
        }
    }

    private async Task RemoveInviteAsync(string inviteeId)
    {
        OnUi(() => InviteError = "");
        try
        {
            await _billing.RemoveInviteAsync(inviteeId);
            await LoadInvitesAsync();
        }
        catch (TyraxException ex)
        {
            OnUi(() => InviteError = ex.Message);
        }
    }

    private static string BuildStatusLine(Subscription sub)
    {
        var ends = string.IsNullOrEmpty(sub.EndsAt) ? "" : $" · ДО {FormatDate(sub.EndsAt)}";
        var devices = $" · {sub.DevicesCount}/{sub.DevicesLimit} УСТР.";
        return $"ТАРИФ {sub.Tier.ToUpperInvariant()}{ends}{devices}";
    }

    private static string FormatDate(string iso)
    {
        if (DateTime.TryParse(iso, out var dt))
            return dt.ToLocalTime().ToString("dd.MM.yyyy");
        return iso.Length >= 10 ? iso[..10] : iso;
    }

    private static void OnUi(Action action)
    {
        var dispatcher = System.Windows.Application.Current?.Dispatcher;
        if (dispatcher is null || dispatcher.CheckAccess()) action();
        else dispatcher.Invoke(action);
    }
}
