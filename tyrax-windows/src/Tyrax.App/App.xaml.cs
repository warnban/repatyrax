using System.Windows;
using Tyrax.App.Services;
using Tyrax.App.ViewModels;
using Tyrax.Data.Remote;
using Tyrax.Data.Repositories;
using Tyrax.Data.Security;
using Tyrax.Data.Session;

namespace Tyrax.App;

/// <summary>
/// Composition root. Wires the secure store → session → API → repositories →
/// shell by hand (the graph is small enough not to need a DI container yet).
/// </summary>
public partial class App : System.Windows.Application
{
    protected override async void OnStartup(StartupEventArgs e)
    {
        base.OnStartup(e);

        var store = new DpapiSecureStore();
        var session = new SessionManager(store);
        var api = TyraxApiFactory.Create(session);
        var authRepo = new AuthRepository(api);
        var vpnRepo = new VpnRepository(api);
        var billingRepo = new BillingRepository(api);
        var ipc = new TunnelIpcClient();

        var shell = new ShellViewModel(session, store, authRepo, vpnRepo, billingRepo, ipc);
        var window = new MainWindow(shell);
        window.Show();

        await shell.InitializeAsync();
    }
}
