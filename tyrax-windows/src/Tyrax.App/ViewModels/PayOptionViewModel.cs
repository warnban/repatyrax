using CommunityToolkit.Mvvm.ComponentModel;

namespace Tyrax.App.ViewModels;

/// <summary>A selectable chip on the payment screen (a month-count or a method).</summary>
public sealed partial class PayOptionViewModel : ObservableObject
{
    [ObservableProperty] private bool _isSelected;

    public PayOptionViewModel(string label, object value)
    {
        Label = label;
        Value = value;
    }

    public string Label { get; }
    public object Value { get; }
}
