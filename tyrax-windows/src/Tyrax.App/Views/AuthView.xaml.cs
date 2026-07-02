using System.Windows;
using System.Windows.Controls;
using Tyrax.App.ViewModels;

namespace Tyrax.App.Views;

/// <summary>
/// IDENTITY gate. <see cref="PasswordBox"/> is not bindable for security reasons,
/// so its value is pushed to the view model on change.
/// </summary>
public partial class AuthView : System.Windows.Controls.UserControl
{
    public AuthView() => InitializeComponent();

    private void PasswordBox_OnPasswordChanged(object sender, RoutedEventArgs e)
    {
        if (DataContext is AuthViewModel vm && sender is PasswordBox box)
            vm.Password = box.Password;
    }
}
