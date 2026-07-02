using Tyrax.Core.Models;
using SolidColorBrush = System.Windows.Media.SolidColorBrush;
using Color = System.Windows.Media.Color;

namespace Tyrax.App.ViewModels;

/// <summary>Строка локации в списке.</summary>
public sealed class NodeItemViewModel
{
    private static readonly SolidColorBrush White = new(Color.FromRgb(0xFF, 0xFF, 0xFF));
    private static readonly SolidColorBrush Red = new(Color.FromRgb(0xFF, 0x1E, 0x1E));
    private static readonly SolidColorBrush Dim = new(Color.FromRgb(0x6E, 0x6E, 0x6E));

    public NodeItemViewModel(Node node) => Node = node;

    public Node Node { get; }

    public string Codename => Node.Codename.ToUpperInvariant();
    public string Status => LocalizeStatus(Node.Status);
    public string PingText => Node.PingMs > 0 ? $"{Node.PingMs} МС" : "— МС";
    public string LoadText => Node.Load >= 0 ? $"НАГР {Node.Load}" : "";

    public System.Windows.Media.Brush StatusBrush => Node.Status.ToUpperInvariant() switch
    {
        "OPEN" => White,
        "MONITORED" => Red,
        _ => Dim,
    };

    public bool IsSelectable => string.Equals(Node.Status, "OPEN", StringComparison.OrdinalIgnoreCase);

    private static string LocalizeStatus(string status) => status.ToUpperInvariant() switch
    {
        "OPEN" => "ОТКРЫТА",
        "MONITORED" => "ПОД НАБЛЮДЕНИЕМ",
        "HEAVILY RESTRICTED" => "ОГРАНИЧЕНА",
        _ => status.ToUpperInvariant(),
    };
}
