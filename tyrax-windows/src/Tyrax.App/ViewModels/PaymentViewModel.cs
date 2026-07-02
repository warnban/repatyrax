using System.Collections.ObjectModel;
using System.Diagnostics;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Tyrax.Core;
using Tyrax.Core.Abstractions;
using Tyrax.Core.Models;

namespace Tyrax.App.ViewModels;

/// <summary>Оплата тарифа.</summary>
public sealed partial class PaymentViewModel : ObservableObject
{
    private static readonly (string Code, string Label)[] MethodDefs =
    {
        ("CARD_RF", "КАРТА"),
        ("SBP", "СБП"),
        ("CRYPTO", "КРИПТО"),
    };

    private readonly IBillingRepository _billing;
    private readonly IAuthRepository _auth;

    [ObservableProperty] private string _tier = "CORE";
    [ObservableProperty] private int _selectedMonths = 1;
    [ObservableProperty] private string _selectedMethod = "SBP";
    [ObservableProperty] private string _totalText = "";
    [ObservableProperty] private string _monthlyText = "";
    [ObservableProperty] private string _savingText = "";
    [ObservableProperty] private string _statusText = "";
    [ObservableProperty] private string _error = "";
    [ObservableProperty] private bool _busy;
    [ObservableProperty] private bool _success;

    public ObservableCollection<PayOptionViewModel> MonthOptions { get; } = new();
    public ObservableCollection<PayOptionViewModel> MethodOptions { get; } = new();

    public PaymentViewModel(IBillingRepository billing, IAuthRepository auth)
    {
        _billing = billing;
        _auth = auth;
        BuildOptions();
        Recompute();
    }

    public event Action? BackRequested;
    public event Action? Paid;

    public void Init(string tier)
    {
        Tier = tier.ToUpperInvariant();
        Success = false;
        Error = "";
        StatusText = "";
        Busy = false;

        if (MonthOptions.Count == 0 || MethodOptions.Count == 0) BuildOptions();
        SelectOne(MonthOptions, o => o.Value is int m && m == 1);
        SelectOne(MethodOptions, o => o.Value is string s && s == "SBP");

        SelectedMonths = 1;
        SelectedMethod = "SBP";
        Recompute();
    }

    private void BuildOptions()
    {
        MonthOptions.Clear();
        foreach (var m in BillingPlan.Months)
            MonthOptions.Add(new PayOptionViewModel($"{m} МЕС", m) { IsSelected = m == 1 });

        MethodOptions.Clear();
        foreach (var (code, label) in MethodDefs)
            MethodOptions.Add(new PayOptionViewModel(label, code) { IsSelected = code == "SBP" });
    }

    private static void SelectOne(ObservableCollection<PayOptionViewModel> options, Func<PayOptionViewModel, bool> match)
    {
        foreach (var o in options) o.IsSelected = match(o);
    }

    [RelayCommand]
    private void Back() => BackRequested?.Invoke();

    [RelayCommand]
    private void ChooseMonths(PayOptionViewModel? option)
    {
        if (option is null || option.Value is not int months) return;
        foreach (var o in MonthOptions) o.IsSelected = ReferenceEquals(o, option);
        SelectedMonths = months;
        Recompute();
    }

    [RelayCommand]
    private void ChooseMethod(PayOptionViewModel? option)
    {
        if (option is null || option.Value is not string code) return;
        foreach (var o in MethodOptions) o.IsSelected = ReferenceEquals(o, option);
        SelectedMethod = code;
    }

    private void Recompute()
    {
        var (total, monthly, saving) = BillingPlan.Quote(Tier, SelectedMonths);
        TotalText = $"ИТОГО {total} ₽";
        MonthlyText = $"≈ {monthly} ₽/МЕС";
        SavingText = saving > 0 ? $"ЭКОНОМИЯ {saving} ₽" : "";
    }

    [RelayCommand]
    private async Task PayAsync()
    {
        if (Busy) return;
        OnUi(() => { Busy = true; Error = ""; StatusText = "СОЗДАНИЕ ЗАКАЗА…"; });
        try
        {
            var profile = await _auth.GetProfileAsync();
            var email = profile.Email ?? "";
            if (email.Length == 0)
            {
                OnUi(() => { Error = "НУЖЕН EMAIL. ПРИВЯЖИ В TELEGRAM."; Busy = false; StatusText = ""; });
                return;
            }

            var result = await _billing.CreatePaymentAsync(Tier, SelectedMethod, SelectedMonths, email);
            OpenUrl(result.PaymentUrl);
            OnUi(() => StatusText = "ОЖИДАНИЕ ОПЛАТЫ…");
            await PollAsync(result.OrderId);
        }
        catch (TyraxException ex)
        {
            OnUi(() => { Error = ex.Message; StatusText = ""; });
        }
        finally
        {
            OnUi(() => Busy = false);
        }
    }

    private async Task PollAsync(string orderId)
    {
        for (var i = 0; i < 100; i++)
        {
            await Task.Delay(3000);
            try
            {
                var status = await _billing.GetPaymentStatusAsync(orderId);
                if (string.Equals(status.OrderStatus, "PAID", StringComparison.OrdinalIgnoreCase))
                {
                    OnUi(() => { Success = true; StatusText = "ДОСТУП ОТКРЫТ"; });
                    Paid?.Invoke();
                    return;
                }
            }
            catch (TyraxException) { }
        }
        OnUi(() => StatusText = "ОПЛАТА В ОБРАБОТКЕ. ПРОВЕРЬ ПОЗЖЕ.");
    }

    private static void OpenUrl(string url)
    {
        try { Process.Start(new ProcessStartInfo(url) { UseShellExecute = true }); }
        catch (Exception) { }
    }

    private static void OnUi(Action action)
    {
        var dispatcher = System.Windows.Application.Current?.Dispatcher;
        if (dispatcher is null || dispatcher.CheckAccess()) action();
        else dispatcher.Invoke(action);
    }
}
