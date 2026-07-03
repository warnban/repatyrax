using System.Windows.Controls;
using Tyrax.App.ViewModels;

namespace Tyrax.App.Views;

/// <summary>
/// IDENTITY gate. Password fields are not bindable; values are pushed to the VM on change
/// and read live at command time via <see cref="AuthViewModel.ReadPassword"/>.
/// </summary>
public partial class AuthView : System.Windows.Controls.UserControl
{
    public AuthView()
    {
        InitializeComponent();
        Loaded += OnLoaded;
        DataContextChanged += (_, _) => WirePasswordReaders();
    }

    private void OnLoaded(object sender, System.Windows.RoutedEventArgs e) => WirePasswordReaders();

    private void WirePasswordReaders()
    {
        if (DataContext is not AuthViewModel vm) return;
        vm.ReadPassword = () => (IsLoginMode() ? LoginPasswordBox : PasswordBox).Password;
        vm.ReadConfirmPassword = () => ConfirmPasswordBox.Password;
    }

    private bool IsLoginMode() =>
        DataContext is AuthViewModel vm && vm.IsLoginMode && !vm.VerificationRequired;

    private void PasswordBox_OnPasswordChanged(object sender, System.Windows.RoutedEventArgs e)
    {
        if (DataContext is AuthViewModel vm && sender is PasswordBox box)
            vm.Password = box.Password;
    }

    private void LoginPasswordBox_OnPasswordChanged(object sender, System.Windows.RoutedEventArgs e)
    {
        if (DataContext is AuthViewModel vm && sender is PasswordBox box)
            vm.Password = box.Password;
    }

    private void ConfirmPasswordBox_OnPasswordChanged(object sender, System.Windows.RoutedEventArgs e)
    {
        if (DataContext is AuthViewModel vm && sender is PasswordBox box)
            vm.ConfirmPassword = box.Password;
    }
}
