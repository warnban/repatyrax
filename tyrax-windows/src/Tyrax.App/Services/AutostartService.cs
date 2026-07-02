using Microsoft.Win32;

namespace Tyrax.App.Services;

/// <summary>
/// Per-user launch-at-logon toggle via the HKCU Run key. The installer offers a
/// machine-wide autostart task; this lets the user flip their own preference from
/// SETTINGS without touching the elevated service. Fail-soft: registry errors are
/// swallowed so the UI never crashes over a preference.
/// </summary>
public sealed class AutostartService
{
    private const string RunKey = @"Software\Microsoft\Windows\CurrentVersion\Run";
    private const string ValueName = "TYRAX";

    public bool IsEnabled()
    {
        try
        {
            using var key = Registry.CurrentUser.OpenSubKey(RunKey, writable: false);
            return key?.GetValue(ValueName) is string s && s.Length > 0;
        }
        catch (Exception)
        {
            return false;
        }
    }

    public void SetEnabled(bool enabled)
    {
        try
        {
            using var key = Registry.CurrentUser.CreateSubKey(RunKey, writable: true);
            if (key is null) return;
            if (enabled)
            {
                var exe = Environment.ProcessPath;
                if (!string.IsNullOrEmpty(exe)) key.SetValue(ValueName, $"\"{exe}\"");
            }
            else
            {
                key.DeleteValue(ValueName, throwOnMissingValue: false);
            }
        }
        catch (Exception)
        {
            // preference is best-effort
        }
    }
}
