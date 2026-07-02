using System.ComponentModel;
using System.Windows;
using System.Windows.Input;
using Tyrax.App.ViewModels;
using Drawing = System.Drawing;
using Forms = System.Windows.Forms;

namespace Tyrax.App;

/// <summary>
/// Shell window. Hosts the Onboarding/Auth/Main screens, gated by the session, owns
/// the borderless chrome (drag + window buttons) and lives in the system tray:
/// closing hides to tray, EXIT truly quits so the tunnel keeps running in the
/// background service while the UI is out of the way.
/// </summary>
public partial class MainWindow : Window
{
    private Forms.NotifyIcon? _tray;
    private bool _exit;

    public MainWindow(ShellViewModel shell)
    {
        InitializeComponent();
        DataContext = shell;
        SetupTray();
        Closing += OnClosing;
    }

    private void Header_DragMove(object sender, MouseButtonEventArgs e)
    {
        if (e.ButtonState == MouseButtonState.Pressed) DragMove();
    }

    private void Minimize_Click(object sender, RoutedEventArgs e)
        => WindowState = WindowState.Minimized;

    // Closing the window hides to tray; real quit is EXIT from the tray menu.
    private void Close_Click(object sender, RoutedEventArgs e) => HideToTray();

    private void OnClosing(object? sender, CancelEventArgs e)
    {
        if (_exit) return;
        e.Cancel = true;
        HideToTray();
    }

    // ── System tray ────────────────────────────────────────────────────────────

    private void SetupTray()
    {
        var menu = new Forms.ContextMenuStrip();
        menu.Items.Add("ОТКРЫТЬ TYRAX", null, (_, _) => ShowFromTray());
        menu.Items.Add("ВКЛ/ВЫКЛ", null, (_, _) => Toggle());
        menu.Items.Add(new Forms.ToolStripSeparator());
        menu.Items.Add("ВЫХОД", null, (_, _) => ExitApp());

        _tray = new Forms.NotifyIcon
        {
            Icon = LoadTrayIcon(),
            Text = "TYRAX",
            Visible = true,
            ContextMenuStrip = menu,
        };
        _tray.DoubleClick += (_, _) => ShowFromTray();
    }

    private static Drawing.Icon LoadTrayIcon()
    {
        // Prefer the shipped brand .ico (crisp at tray sizes)...
        try
        {
            var icoPath = System.IO.Path.Combine(AppContext.BaseDirectory, "Assets", "tyrax.ico");
            if (System.IO.File.Exists(icoPath)) return new Drawing.Icon(icoPath, new Drawing.Size(32, 32));
        }
        catch (Exception) { /* fall through */ }

        // ...then the exe's embedded icon, then the OS default.
        try
        {
            var path = Environment.ProcessPath;
            if (path is not null)
            {
                var icon = Drawing.Icon.ExtractAssociatedIcon(path);
                if (icon is not null) return icon;
            }
        }
        catch (Exception) { /* fall through */ }
        return Drawing.SystemIcons.Application;
    }

    private void ShowFromTray()
    {
        Show();
        WindowState = WindowState.Normal;
        ShowInTaskbar = true;
        Activate();
    }

    private void HideToTray()
    {
        Hide();
        ShowInTaskbar = false;
    }

    private void Toggle()
    {
        if (DataContext is ShellViewModel shell && shell.Main.ToggleCommand.CanExecute(null))
            shell.Main.ToggleCommand.Execute(null);
    }

    private void ExitApp()
    {
        _exit = true;
        if (_tray is not null)
        {
            _tray.Visible = false;
            _tray.Dispose();
            _tray = null;
        }
        System.Windows.Application.Current.Shutdown();
    }
}
