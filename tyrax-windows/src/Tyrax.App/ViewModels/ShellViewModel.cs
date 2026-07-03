using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Tyrax.App.Services;
using Tyrax.Core.Abstractions;

namespace Tyrax.App.ViewModels;

/// <summary>Корневой VM: навигация, pipe к службе, подписка/квота на главном экране.</summary>
public sealed partial class ShellViewModel : ObservableObject
{
    private static readonly TimeSpan ReconnectDelay = TimeSpan.FromSeconds(2);
    private const string OnboardedKey = "onboarded";

    private readonly ISession _session;
    private readonly ISecureStore _store;
    private readonly IVpnRepository _vpn;
    private readonly IBillingRepository _billing;
    private readonly TunnelIpcClient _ipc;
    private readonly ConnectionSupervisor _supervisor;
    private readonly UpdateChecker _updates = new();
    private int _reconnecting;

    [ObservableProperty] private bool _isAuthenticated;
    [ObservableProperty] private bool _needsOnboarding;
    [ObservableProperty] private AppScreen _currentScreen = AppScreen.Main;

    public AuthViewModel Auth { get; }
    public MainViewModel Main { get; }
    public NodesViewModel Nodes { get; }
    public DevicesViewModel Devices { get; }
    public SubscriptionViewModel Subscription { get; }
    public PaymentViewModel Payment { get; }
    public SettingsViewModel Settings { get; }
    public OnboardingViewModel Onboarding { get; } = new();

    public ShellViewModel(
        ISession session,
        ISecureStore store,
        IAuthRepository auth,
        IVpnRepository vpn,
        IBillingRepository billing,
        TunnelIpcClient ipc)
    {
        _session = session;
        _store = store;
        _vpn = vpn;
        _billing = billing;
        _ipc = ipc;
        _supervisor = new ConnectionSupervisor(vpn, session, ipc);

        Auth = new AuthViewModel(auth, vpn, session);
        Main = new MainViewModel(_supervisor, ipc);
        Nodes = new NodesViewModel(vpn);
        Devices = new DevicesViewModel(vpn, session);
        Subscription = new SubscriptionViewModel(billing);
        Payment = new PaymentViewModel(billing, auth);
        Settings = new SettingsViewModel(new AutostartService(), session);

        Auth.Authenticated += OnAuthenticated;
        Onboarding.Completed += OnOnboardingDone;

        Main.NodesRequested += OnOpenNodes;
        Main.DevicesRequested += OnOpenDevices;
        Main.ControlRequested += OnOpenControl;
        Main.SettingsRequested += OnOpenSettings;
        Main.ServiceRetryRequested += OnServiceRetry;

        Nodes.BackRequested += () => CurrentScreen = AppScreen.Main;
        Nodes.NodeChosen += OnNodeChosen;

        Devices.BackRequested += () => CurrentScreen = AppScreen.Main;

        Subscription.BackRequested += () => CurrentScreen = AppScreen.Main;
        Subscription.PayRequested += OnPayRequested;

        Payment.BackRequested += () => CurrentScreen = AppScreen.Subscription;
        Payment.Paid += OnPaid;

        Settings.BackRequested += () => CurrentScreen = AppScreen.Main;
        Settings.SignOutRequested += OnSignOut;

        _ipc.Disconnected += OnIpcDropped;
    }

    public async Task InitializeAsync()
    {
        IsAuthenticated = _session.IsAuthenticated;
        NeedsOnboarding = !IsAuthenticated && _store.Get(OnboardedKey) != "1";
        await TryConnectPipeAsync();
        if (IsAuthenticated) await RefreshSubscriptionAsync();
        _ = CheckForUpdateAsync();
    }

    private void OnOnboardingDone()
    {
        _store.Set(OnboardedKey, "1");
        NeedsOnboarding = false;
    }

    private async Task CheckForUpdateAsync()
    {
        var info = await _updates.CheckAsync();
        if (info is not null) Main.SetUpdate(info.Version.ToString(), info.Url);
    }

    private void OnAuthenticated()
    {
        NeedsOnboarding = false;
        IsAuthenticated = true;
        _ = EnsureDeviceRegisteredAsync();
        _ = RefreshSubscriptionAsync();
    }

    private async Task EnsureDeviceRegisteredAsync()
    {
        try { await _vpn.RegisterDeviceAsync(_session.DeviceName); }
        catch (Exception) { /* connect path can still auto-provision */ }
    }

    private async void OnOpenNodes()
    {
        CurrentScreen = AppScreen.Nodes;
        await Nodes.RefreshAsync();
    }

    private async void OnOpenDevices()
    {
        CurrentScreen = AppScreen.Devices;
        await Devices.RefreshAsync();
    }

    private void OnOpenSettings()
    {
        Settings.Load();
        CurrentScreen = AppScreen.Settings;
    }

    private void OnServiceRetry() => _ = TryConnectPipeAsync();

    private async void OnSignOut()
    {
        try { await _supervisor.StopAsync(); } catch (Exception) { }
        _session.SignOut();
        CurrentScreen = AppScreen.Main;
        IsAuthenticated = false;
    }

    private async void OnOpenControl()
    {
        CurrentScreen = AppScreen.Subscription;
        await Subscription.RefreshAsync();
        await RefreshSubscriptionAsync();
    }

    private async void OnNodeChosen(string codename)
    {
        CurrentScreen = AppScreen.Main;
        await Main.ConnectToAsync(codename);
    }

    private void OnPayRequested(string tier)
    {
        Payment.Init(tier);
        CurrentScreen = AppScreen.Payment;
    }

    private async void OnPaid()
    {
        CurrentScreen = AppScreen.Subscription;
        await Subscription.RefreshAsync();
        await RefreshSubscriptionAsync();
    }

    partial void OnCurrentScreenChanged(AppScreen value)
    {
        if (value == AppScreen.Main && IsAuthenticated)
            _ = RefreshSubscriptionAsync();
    }

    private async Task RefreshSubscriptionAsync()
    {
        if (!IsAuthenticated) return;
        try
        {
            var sub = await _billing.GetSubscriptionAsync();
            Main.SetSubscription(sub);
            Subscription.ApplySubscription(sub);
        }
        catch (Exception) { /* квота обновится при следующем заходе в тарифы */ }
    }

    private async Task TryConnectPipeAsync()
    {
        try
        {
            await _ipc.ConnectAsync();
            Main.SetServiceOnline(true);
        }
        catch (Exception)
        {
            Main.SetServiceOnline(false);
            StartReconnectLoop();
        }
    }

    private void OnIpcDropped()
    {
        Main.SetServiceOnline(false);
        StartReconnectLoop();
    }

    private void StartReconnectLoop()
    {
        if (Interlocked.CompareExchange(ref _reconnecting, 1, 0) != 0) return;
        _ = Task.Run(async () =>
        {
            try
            {
                while (true)
                {
                    await Task.Delay(ReconnectDelay);
                    try
                    {
                        await _ipc.ConnectAsync();
                        Main.SetServiceOnline(true);
                        return;
                    }
                    catch (Exception) { }
                }
            }
            finally
            {
                Interlocked.Exchange(ref _reconnecting, 0);
            }
        });
    }
}
