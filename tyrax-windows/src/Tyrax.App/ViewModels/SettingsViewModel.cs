using System.Diagnostics;
using System.Reflection;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Tyrax.App.Services;
using Tyrax.Core.Abstractions;

namespace Tyrax.App.ViewModels;

/// <summary>Настройки: автозапуск, устройство, версия, выход.</summary>
public sealed partial class SettingsViewModel : ObservableObject
{
    private const string BotUrl = "https://t.me/tyraxvpnbot";
    private const string PrivacyUrl = "https://tyrax.tech/privacy.html";
    private const string TermsUrl = "https://tyrax.tech/terms.html";
    private const string SupportUrl = "https://tyrax.tech/contacts.html";

    private readonly AutostartService _autostart;
    private readonly ISession _session;

    [ObservableProperty] private bool _autostartEnabled;
    [ObservableProperty] private string _deviceLine = "";
    [ObservableProperty] private string _versionLine = "";

    private bool _applying;

    public SettingsViewModel(AutostartService autostart, ISession session)
    {
        _autostart = autostart;
        _session = session;

        var v = Assembly.GetExecutingAssembly().GetName().Version ?? new Version(1, 0);
        VersionLine = $"TYRAX v{v.Major}.{v.Minor}.{v.Build}";
        DeviceLine = $"УСТРОЙСТВО: {_session.DeviceName}";
    }

    public event Action? BackRequested;
    public event Action? SignOutRequested;

    public void Load()
    {
        _applying = true;
        AutostartEnabled = _autostart.IsEnabled();
        DeviceLine = $"УСТРОЙСТВО: {_session.DeviceName}";
        _applying = false;
    }

    partial void OnAutostartEnabledChanged(bool value)
    {
        if (_applying) return;
        _autostart.SetEnabled(value);
    }

    [RelayCommand]
    private void Back() => BackRequested?.Invoke();

    [RelayCommand]
    private void OpenBot()
    {
        try { Process.Start(new ProcessStartInfo(BotUrl) { UseShellExecute = true }); }
        catch (Exception) { }
    }

    [RelayCommand]
    private void OpenPrivacy()
    {
        try { Process.Start(new ProcessStartInfo(PrivacyUrl) { UseShellExecute = true }); }
        catch (Exception) { }
    }

    [RelayCommand]
    private void OpenTerms()
    {
        try { Process.Start(new ProcessStartInfo(TermsUrl) { UseShellExecute = true }); }
        catch (Exception) { }
    }

    [RelayCommand]
    private void OpenSupport()
    {
        try { Process.Start(new ProcessStartInfo(SupportUrl) { UseShellExecute = true }); }
        catch (Exception) { }
    }

    [RelayCommand]
    private void SignOut() => SignOutRequested?.Invoke();
}
