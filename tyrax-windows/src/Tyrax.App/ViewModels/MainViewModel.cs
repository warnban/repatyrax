using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Tyrax.App.Services;
using Tyrax.Core.Models;
using Tyrax.Ipc;

namespace Tyrax.App.ViewModels;

/// <summary>Главный экран: статус туннеля, квота, цифровой дождь, навигация.</summary>
public sealed partial class MainViewModel : ObservableObject
{
    private readonly ConnectionSupervisor _supervisor;
    private readonly TunnelIpcClient _ipc;

    [ObservableProperty] private TunnelState _state = TunnelState.Disconnected;
    [ObservableProperty] private string _statusText = "ВНЕ СИСТЕМЫ";
    [ObservableProperty] private string _nodeTag = "ЛОКАЦИЯ: —";
    [ObservableProperty] private string _actionText = "ВКЛЮЧИТЬ";
    [ObservableProperty] private string _trafficText = "";
    [ObservableProperty] private bool _serviceOnline;
    [ObservableProperty] private string _serviceStatusText = "ПОДКЛЮЧЕНИЕ К ДВИЖКУ…";
    [ObservableProperty] private bool _isBreaching;
    [ObservableProperty] private bool _isRainActive;
    [ObservableProperty] private string _updateText = "";
    [ObservableProperty] private string _quotaText = "";
    [ObservableProperty] private bool _showQuota;
    [ObservableProperty] private string _blockedText = "";
    [ObservableProperty] private bool _showBlockedBanner;

    private string? _updateUrl;

    public MainViewModel(ConnectionSupervisor supervisor, TunnelIpcClient ipc)
    {
        _supervisor = supervisor;
        _ipc = ipc;
        _ipc.StatusReceived += OnStatus;
        _ipc.Disconnected += OnPipeDropped;
    }

    public event Action? NodesRequested;
    public event Action? ControlRequested;
    public event Action? DevicesRequested;
    public event Action? SettingsRequested;
    public event Action? ServiceRetryRequested;

    public void SetServiceOnline(bool online) => OnUi(() =>
    {
        ServiceOnline = online;
        ServiceStatusText = online ? "СИСТЕМА ГОТОВА" : "СЛУЖБА НЕДОСТУПНА — ПЕРЕПОДКЛЮЧЕНИЕ…";
        if (!online && State != TunnelState.Connected) StatusText = "ВНЕ СИСТЕМЫ";
    });

    /// <summary>Обновляет строку квоты и баннер блокировки с экрана тарифов / при старте.</summary>
    public void SetSubscription(Subscription sub) => OnUi(() =>
    {
        if (!string.IsNullOrEmpty(sub.BlockedUntil))
        {
            var date = FormatDate(sub.BlockedUntil);
            BlockedText = $"ДОСТУП ИСЧЕРПАН · ДО {date}";
            ShowBlockedBanner = true;
        }
        else
        {
            ShowBlockedBanner = false;
            BlockedText = "";
        }

        if (sub.Unlimited || sub.TrafficLimitBytes <= 0)
        {
            QuotaText = sub.Unlimited ? "ТРАФИК · БЕЗЛИМИТ" : "";
            ShowQuota = sub.Unlimited;
        }
        else
        {
            var usedGb = sub.TrafficUsedBytes / 1024.0 / 1024.0 / 1024.0;
            var limitGb = sub.TrafficLimitBytes / 1024.0 / 1024.0 / 1024.0;
            var remaining = Math.Max(0, limitGb - usedGb);
            QuotaText = $"ОСТАЛОСЬ {remaining:0.0} ГБ ИЗ {limitGb:0.0} ГБ";
            ShowQuota = true;
        }
    });

    public void SetUpdate(string version, string url) => OnUi(() =>
    {
        _updateUrl = url;
        UpdateText = $"ОБНОВЛЕНИЕ {version} — УСТАНОВИТЬ";
    });

    [RelayCommand]
    private void OpenNodes() => NodesRequested?.Invoke();

    [RelayCommand]
    private void OpenControl() => ControlRequested?.Invoke();

    [RelayCommand]
    private void OpenDevices() => DevicesRequested?.Invoke();

    [RelayCommand]
    private void OpenSettings() => SettingsRequested?.Invoke();

    [RelayCommand]
    private void OpenUpdate()
    {
        if (string.IsNullOrEmpty(_updateUrl)) return;
        try { System.Diagnostics.Process.Start(new System.Diagnostics.ProcessStartInfo(_updateUrl) { UseShellExecute = true }); }
        catch (Exception) { }
    }

    [RelayCommand]
    private void OpenTariffsFromBanner() => ControlRequested?.Invoke();

    [RelayCommand]
    private async Task ToggleAsync()
    {
        if (!ServiceOnline)
        {
            StatusText = "СЛУЖБА НЕДОСТУПНА — ПЕРЕПОДКЛЮЧЕНИЕ…";
            ServiceRetryRequested?.Invoke();
            return;
        }

        if (State is TunnelState.Connected or TunnelState.Connecting)
            await _supervisor.StopAsync();
        else
            _supervisor.Start(null);
    }

    public Task ConnectToAsync(string? codename)
    {
        if (ServiceOnline) _supervisor.Start(codename);
        return Task.CompletedTask;
    }

    private void OnStatus(IpcStatus s) => OnUi(() =>
    {
        State = s.State;
        IsBreaching = s.State == TunnelState.Connecting;
        IsRainActive = s.State == TunnelState.Connected;
        NodeTag = s.Codename is null ? "ЛОКАЦИЯ: —" : $"ЛОКАЦИЯ: {s.Codename.ToUpperInvariant()} · ОТКРЫТА";
        StatusText = s.State switch
        {
            TunnelState.Disconnected => "ВНЕ СИСТЕМЫ",
            TunnelState.Connecting => "ПРОНИКНОВЕНИЕ В СЕТЬ…",
            TunnelState.Connected => "ДОСТУП ОТКРЫТ",
            TunnelState.Disconnecting => "ВЫХОД ИЗ СИСТЕМЫ…",
            TunnelState.Error => s.Message ?? "СБОЙ ПОДКЛЮЧЕНИЯ",
            _ => StatusText,
        };
        ActionText = s.State is TunnelState.Connected or TunnelState.Connecting ? "ВЫКЛЮЧИТЬ" : "ВКЛЮЧИТЬ";
        TrafficText = s.State == TunnelState.Connected
            ? $"ОТПР {FormatBytes(s.TxBytes)} · ПРИЁМ {FormatBytes(s.RxBytes)}"
            : "";
    });

    private void OnPipeDropped() => OnUi(() =>
    {
        ServiceOnline = false;
        ServiceStatusText = "СЛУЖБА НЕДОСТУПНА — ПЕРЕПОДКЛЮЧЕНИЕ…";
        State = TunnelState.Disconnected;
        StatusText = "ВНЕ СИСТЕМЫ";
        ActionText = "ВКЛЮЧИТЬ";
        TrafficText = "";
        IsRainActive = false;
    });

    private static string FormatDate(string iso)
    {
        if (DateTime.TryParse(iso, out var dt))
            return dt.ToLocalTime().ToString("dd.MM.yyyy HH:mm");
        return iso.Length >= 10 ? iso[..10] : iso;
    }

    private static string FormatBytes(long bytes)
    {
        string[] units = { "Б", "КБ", "МБ", "ГБ", "ТБ" };
        double value = bytes;
        int i = 0;
        while (value >= 1024 && i < units.Length - 1) { value /= 1024; i++; }
        return $"{value:0.0} {units[i]}";
    }

    private static void OnUi(Action action)
    {
        var dispatcher = System.Windows.Application.Current?.Dispatcher;
        if (dispatcher is null || dispatcher.CheckAccess()) action();
        else dispatcher.Invoke(action);
    }
}
