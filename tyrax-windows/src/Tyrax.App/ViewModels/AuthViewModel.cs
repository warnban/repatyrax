using System.Diagnostics;

using CommunityToolkit.Mvvm.ComponentModel;

using CommunityToolkit.Mvvm.Input;

using Tyrax.Core;

using Tyrax.Core.Abstractions;

using Tyrax.Core.Models;



namespace Tyrax.App.ViewModels;



/// <summary>Регистрация / вход / подтверждение email.</summary>

public sealed partial class AuthViewModel : ObservableObject

{

    private readonly IAuthRepository _auth;

    private readonly IVpnRepository _vpn;

    private readonly ISession _session;



    [ObservableProperty] private string _email = "";

    [ObservableProperty] private string _password = "";

    [ObservableProperty] private string _confirmPassword = "";

    [ObservableProperty] private string _error = "";

    [ObservableProperty] private bool _busy;

    [ObservableProperty] private string _telegramHint = "";



    /// <summary>Register by default; login when user taps the switch link.</summary>

    [ObservableProperty] private bool _isLoginMode;



    [ObservableProperty] private bool _verificationRequired;

    [ObservableProperty] private string _code = "";

    [ObservableProperty] private string _info = "";



    public AuthViewModel(IAuthRepository auth, IVpnRepository vpn, ISession session)

    {

        _auth = auth;

        _vpn = vpn;

        _session = session;

    }



    public bool IsRegisterMode => !IsLoginMode && !VerificationRequired;



    public Func<string>? ReadPassword { get; set; }

    public Func<string>? ReadConfirmPassword { get; set; }



    public event Action? Authenticated;



    partial void OnIsLoginModeChanged(bool value) => OnPropertyChanged(nameof(IsRegisterMode));

    partial void OnVerificationRequiredChanged(bool value) => OnPropertyChanged(nameof(IsRegisterMode));



    [RelayCommand]

    private void SwitchToLogin()

    {

        IsLoginMode = true;

        Error = "";

        Info = "";

        ConfirmPassword = "";

    }



    [RelayCommand]

    private void SwitchToRegister()

    {

        IsLoginMode = false;

        Error = "";

        Info = "";

        ConfirmPassword = "";

    }



    [RelayCommand]

    private Task LoginAsync() => RunCredentialAuthAsync((email, password) => _auth.LoginAsync(email, password));



    [RelayCommand]

    private Task RegisterAsync()

    {

        if (!TryValidateRegister(out var email, out var password))

            return Task.CompletedTask;

        return RunAuthAsync(async () =>

        {

            var result = await _auth.RegisterAsync(email, password);

            ApplyVerifyInfo(result.EmailSent);

            return result;

        });

    }



    [RelayCommand]

    private Task VerifyAsync() => RunAuthAsync(async () =>

    {

        var email = Email.Trim().ToLowerInvariant();

        var code = new string(Code.Where(char.IsDigit).ToArray());

        if (string.IsNullOrWhiteSpace(email) || string.IsNullOrWhiteSpace(code))

            throw new TyraxException("INVALID OR EXPIRED CODE");

        return await _auth.VerifyEmailAsync(email, code);

    });



    [RelayCommand]

    private async Task ResendAsync()

    {

        if (Busy) return;

        Busy = true;

        Error = "";

        try

        {

            var sent = await _auth.ResendVerificationAsync(Email.Trim());

            ApplyVerifyInfo(sent);

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



    [RelayCommand]

    private void BackToLogin()

    {

        VerificationRequired = false;

        IsLoginMode = true;

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



    private string CurrentPassword()

    {

        var live = ReadPassword?.Invoke();

        return string.IsNullOrEmpty(live) ? Password : live;

    }



    private string CurrentConfirmPassword()

    {

        var live = ReadConfirmPassword?.Invoke();

        return string.IsNullOrEmpty(live) ? ConfirmPassword : live;

    }



    private bool TryValidateCredentials(out string email, out string password)

    {

        email = Email.Trim();

        password = CurrentPassword();

        if (string.IsNullOrWhiteSpace(email) || string.IsNullOrWhiteSpace(password))

        {

            Error = "INVALID CREDENTIALS";

            return false;

        }

        return true;

    }



    private bool TryValidateRegister(out string email, out string password)

    {

        if (!TryValidateCredentials(out email, out password))

            return false;



        var confirm = CurrentConfirmPassword();

        if (password != confirm)

        {

            Error = "ПАРОЛИ НЕ СОВПАДАЮТ";

            return false;

        }

        return true;

    }



    private Task RunCredentialAuthAsync(Func<string, string, Task<AuthResult>> op)

    {

        if (!TryValidateCredentials(out var email, out var password))

            return Task.CompletedTask;

        return RunAuthAsync(async () =>

        {

            var result = await op(email, password);

            if (result.VerificationRequired)

                ApplyVerifyInfo(result.EmailSent);

            return result;

        });

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

                Code = "";

                VerificationRequired = true;

                return;

            }

            await CompleteAsync(result.Token, result.UserId);

        }

        catch (TyraxException ex)

        {

            Error = ex.Message;

            if (VerificationRequired && ex.Message.Contains("INVALID OR EXPIRED", StringComparison.OrdinalIgnoreCase))

                Info = "ПОДТВЕРЖДАЛ ЧЕРЕЗ ПИСЬМО? НАЖМИ НАЗАД И ВОЙДИ С ПАРОЛЕМ";

        }

        finally

        {

            Busy = false;

        }

    }



    private void ApplyVerifyInfo(bool emailSent)

    {

        Info = emailSent

            ? $"КОД ОТПРАВЛЕН НА {Email.Trim().ToUpperInvariant()}"

            : "ПИСЬМО НЕ УШЛО. ПОПРОБУЙ СНОВА ИЛИ НАПИШИ support@tyrax.tech";

    }



    private async Task CompleteAsync(string token, string? userId)

    {

        _session.SignIn(token, userId);

        try { await _vpn.RegisterDeviceAsync(_session.DeviceName); }

        catch (TyraxException) { }

        Authenticated?.Invoke();

    }

}


