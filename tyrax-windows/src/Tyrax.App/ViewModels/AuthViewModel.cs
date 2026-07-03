using System.Diagnostics;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Tyrax.Core;
using Tyrax.Core.Abstractions;
using Tyrax.Core.Models;

namespace Tyrax.App.ViewModels;

/// <summary>Вход: email/пароль и Telegram.</summary>
public sealed partial class AuthViewModel : ObservableObject
{
    private readonly IAuthRepository _auth;
    private readonly IVpnRepository _vpn;
    private readonly ISession _session;

    [ObservableProperty] private string _email = "";
    [ObservableProperty] private string _password = "";
    [ObservableProperty] private string _error = "";
    [ObservableProperty] private bool _busy;
    [ObservableProperty] private string _telegramHint = "";

    // Email confirmation gate. When true the view swaps the credentials panel for
    // the "enter code" panel.
    [ObservableProperty] private bool _verificationRequired;
    [ObservableProperty] private string _code = "";
    [ObservableProperty] private string _info = "";

    public AuthViewModel(IAuthRepository auth, IVpnRepository vpn, ISession session)
    {
        _auth = auth;
        _vpn = vpn;
        _session = session;
    }

    public event Action? Authenticated;

    [RelayCommand]
    private Task LoginAsync() => RunAuthAsync(() => _auth.LoginAsync(Email.Trim(), Password));

    [RelayCommand]
    private Task RegisterAsync() => RunAuthAsync(() => _auth.RegisterAsync(Email.Trim(), Password));

    [RelayCommand]
    private Task VerifyAsync() => RunAuthAsync(() => _auth.VerifyEmailAsync(Email.Trim(), Code.Trim()));

    [RelayCommand]
    private async Task ResendAsync()
    {
        await _auth.ResendVerificationAsync(Email.Trim());
        Info = "КОД ОТПРАВЛЕН ПОВТОРНО";
    }

    [RelayCommand]
    private void BackToLogin()
    {
        VerificationRequired = false;
        Code = "";
        Error = "";
        Info = "";
    }

    [RelayCommand]
    private async Task TelegramAsync()
    {
        if (Busy) return;
        Busy = true;
        Error = "";
        try
        {
            var init = await _auth.TelegramInitAsync();
            TelegramHint = "ПОДТВЕРДИ В TELEGRAM…";
            Process.Start(new ProcessStartInfo(init.BotUrl) { UseShellExecute = true });

            var deadline = DateTime.UtcNow.AddMinutes(2);
            while (DateTime.UtcNow < deadline)
            {
                await Task.Delay(2000);
                var result = await _auth.TelegramStatusAsync(init.Token);
                if (result is not null)
                {
                    await CompleteAsync(result.Token, result.UserId);
                    return;
                }
            }
            Error = "TELEGRAM: ВРЕМЯ ВЫШЛО";
            TelegramHint = "";
        }
        catch (TyraxException ex)
        {
            Error = ex.Message;
            TelegramHint = "";
        }
        finally
        {
            Busy = false;
        }
    }

    private async Task RunAuthAsync(Func<Task<AuthResult>> op)
    {
        if (Busy) return;
        Busy = true;
        Error = "";
        try
        {
            var result = await op();
            if (result.VerificationRequired)
            {
                // No session yet — reveal the "enter code" panel.
                Info = "";
                Code = "";
                VerificationRequired = true;
                return;
            }
            await CompleteAsync(result.Token, result.UserId);
        }
        catch (TyraxException ex)
        {
            Error = ex.Message;
        }
        finally
        {
            Busy = false;
        }
    }

    private async Task CompleteAsync(string token, string? userId)
    {
        _session.SignIn(token, userId);
        try { await _vpn.RegisterDeviceAsync(_session.DeviceName); }
        catch (TyraxException) { }
        Authenticated?.Invoke();
    }
}
