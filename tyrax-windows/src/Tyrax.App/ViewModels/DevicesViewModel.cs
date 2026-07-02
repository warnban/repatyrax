using System.Collections.ObjectModel;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Tyrax.Core;
using Tyrax.Core.Abstractions;

namespace Tyrax.App.ViewModels;

/// <summary>Список устройств аккаунта.</summary>
public sealed partial class DevicesViewModel : ObservableObject
{
    private readonly IVpnRepository _vpn;
    private readonly ISession _session;

    [ObservableProperty] private bool _busy;
    [ObservableProperty] private string _error = "";
    [ObservableProperty] private string _countLine = "";

    public ObservableCollection<UserDeviceItemViewModel> Devices { get; } = new();

    public DevicesViewModel(IVpnRepository vpn, ISession session)
    {
        _vpn = vpn;
        _session = session;
    }

    public event Action? BackRequested;

    [RelayCommand]
    private void Back() => BackRequested?.Invoke();

    [RelayCommand]
    public async Task RefreshAsync()
    {
        if (Busy) return;
        OnUi(() => { Busy = true; Error = ""; });
        try
        {
            var list = await _vpn.GetDevicesAsync();
            var current = _session.DeviceName;
            OnUi(() =>
            {
                Devices.Clear();
                foreach (var d in list)
                {
                    var isCurrent = string.Equals(d.Name, current, StringComparison.OrdinalIgnoreCase);
                    Devices.Add(new UserDeviceItemViewModel(d, isCurrent, id => _ = RevokeAsync(id)));
                }
                CountLine = $"{Devices.Count} УСТРОЙСТВ(А) ПРИВЯЗАНО";
            });
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

    private async Task RevokeAsync(string deviceId)
    {
        OnUi(() => Error = "");
        try
        {
            await _vpn.DeleteDeviceAsync(deviceId);
            await RefreshAsync();
        }
        catch (TyraxException ex)
        {
            OnUi(() => Error = ex.Message);
        }
    }

    private static void OnUi(Action action)
    {
        var dispatcher = System.Windows.Application.Current?.Dispatcher;
        if (dispatcher is null || dispatcher.CheckAccess()) action();
        else dispatcher.Invoke(action);
    }
}
