using System.Collections.ObjectModel;
using System.Windows;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Tyrax.Core;
using Tyrax.Core.Abstractions;

namespace Tyrax.App.ViewModels;

/// <summary>
/// NODES screen: lists exit nodes (codename, ping, load, status badge) and lets
/// the user breach a specific OPEN node instead of the auto-best pick.
/// </summary>
public sealed partial class NodesViewModel : ObservableObject
{
    private readonly IVpnRepository _vpn;

    [ObservableProperty] private bool _busy;
    [ObservableProperty] private string _error = "";

    public ObservableCollection<NodeItemViewModel> Nodes { get; } = new();

    public NodesViewModel(IVpnRepository vpn) => _vpn = vpn;

    /// <summary>Raised to return to the Main screen.</summary>
    public event Action? BackRequested;

    /// <summary>Raised with the chosen node codename to trigger a connect.</summary>
    public event Action<string>? NodeChosen;

    [RelayCommand]
    private void Back() => BackRequested?.Invoke();

    [RelayCommand]
    private void Choose(NodeItemViewModel? item)
    {
        if (item is { IsSelectable: true })
            NodeChosen?.Invoke(item.Codename);
    }

    [RelayCommand]
    public async Task RefreshAsync()
    {
        if (Busy) return;
        Busy = true;
        Error = "";
        try
        {
            var list = await _vpn.GetNodesAsync();
            OnUi(() =>
            {
                Nodes.Clear();
                foreach (var n in list) Nodes.Add(new NodeItemViewModel(n));
            });
        }
        catch (TyraxException ex)
        {
            OnUi(() => Error = ex.Message);
        }
        finally
        {
            Busy = false;
        }
    }

    private static void OnUi(Action action)
    {
        var dispatcher = System.Windows.Application.Current?.Dispatcher;
        if (dispatcher is null || dispatcher.CheckAccess()) action();
        else dispatcher.Invoke(action);
    }
}
