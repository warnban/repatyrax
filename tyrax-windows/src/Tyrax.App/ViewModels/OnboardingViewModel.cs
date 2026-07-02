using System.Diagnostics;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;

namespace Tyrax.App.ViewModels;

/// <summary>Первый запуск — три слайда, затем вход.</summary>
public sealed partial class OnboardingViewModel : ObservableObject
{
    private const string BotUrl = "https://t.me/tyraxvpnbot";
    private const int LastSlide = 2;

    [ObservableProperty] private int _index;
    [ObservableProperty] private string _actionLabel = "ДАЛЕЕ";
    [ObservableProperty] private string _stepText = "01 / 03";

    public event Action? Completed;

    [RelayCommand]
    private void Next()
    {
        if (Index >= LastSlide) { Completed?.Invoke(); return; }
        Index++;
    }

    [RelayCommand]
    private void Skip() => Completed?.Invoke();

    [RelayCommand]
    private void OpenBot()
    {
        try { Process.Start(new ProcessStartInfo(BotUrl) { UseShellExecute = true }); }
        catch (Exception) { }
    }

    partial void OnIndexChanged(int value)
    {
        ActionLabel = value >= LastSlide ? "ВОЙТИ" : "ДАЛЕЕ";
        StepText = $"{value + 1:00} / 03";
    }
}
