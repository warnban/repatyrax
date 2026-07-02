using CommunityToolkit.Mvvm.Input;
using Tyrax.Core.Models;

namespace Tyrax.App.ViewModels;

/// <summary>Одно устройство в списке.</summary>
public sealed class UserDeviceItemViewModel
{
    public UserDeviceItemViewModel(UserDevice device, bool isCurrent, Action<string> onRevoke)
    {
        Id = device.Id;
        Name = device.Name;
        IsCurrent = isCurrent;
        Meta = BuildMeta(device, isCurrent);
        RevokeCommand = new RelayCommand(() => onRevoke(Id), () => !isCurrent);
    }

    public string Id { get; }
    public string Name { get; }
    public bool IsCurrent { get; }
    public string Meta { get; }
    public bool CanRevoke => !IsCurrent;
    public IRelayCommand RevokeCommand { get; }

    private static string BuildMeta(UserDevice d, bool isCurrent)
    {
        var added = string.IsNullOrEmpty(d.CreatedAt) ? "" : $"ДОБАВЛЕНО {d.CreatedAt[..Math.Min(10, d.CreatedAt.Length)]}";
        var tag = isCurrent ? "ЭТО УСТРОЙСТВО" : "";
        return string.Join("  ·  ", new[] { tag, added }.Where(s => s.Length > 0));
    }
}
