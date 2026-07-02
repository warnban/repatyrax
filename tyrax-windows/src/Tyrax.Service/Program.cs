using Tyrax.Service;

// TyraxService — the privileged half of the Windows client. Runs as a Windows
// Service (LocalSystem) so it can own the WinTun adapter, routes and DNS, and
// drive xray.exe + tun2socks. The unprivileged WPF UI talks to it over a named
// pipe. UseWindowsService() is a no-op when launched from a console, so the same
// binary is debuggable directly (F5) and installable as a service.
var builder = Host.CreateApplicationBuilder(args);

builder.Services.AddWindowsService(options =>
{
    options.ServiceName = "TyraxProtocol";
});

builder.Services.AddSingleton<TunnelSupervisor>();
builder.Services.AddHostedService<IpcServer>();

var host = builder.Build();
host.Run();
