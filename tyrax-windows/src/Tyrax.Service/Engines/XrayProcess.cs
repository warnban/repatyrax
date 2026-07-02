using System.Diagnostics;
using System.IO;
using System.Net.NetworkInformation;
using Microsoft.Extensions.Logging;
using Tyrax.Tunnel;

namespace Tyrax.Service.Engines;

/// <summary>
/// Owns a single <c>xray.exe</c> instance: writes the adapted config, spawns
/// the process with the geo databases resolved, and waits until the WinTun
/// adapter created by Xray's TUN inbound is present.
/// </summary>
public sealed class XrayProcess : IAsyncDisposable
{
    private readonly EnginePaths _paths;
    private readonly ILogger _logger;
    private Process? _process;

    public XrayProcess(EnginePaths paths, ILogger logger)
    {
        _paths = paths;
        _logger = logger;
    }

    public bool IsRunning => _process is { HasExited: false };

    /// <summary>Writes <paramref name="configJson"/>, starts xray, waits for the TUN adapter.</summary>
    public async Task StartAsync(string configJson, CancellationToken ct)
    {
        await File.WriteAllTextAsync(_paths.XrayConfig, configJson, ct);

        var psi = new ProcessStartInfo
        {
            FileName = _paths.XrayExe,
            Arguments = $"run -c \"{_paths.XrayConfig}\"",
            WorkingDirectory = _paths.EnginesDir,
            UseShellExecute = false,
            CreateNoWindow = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
        };
        psi.Environment["XRAY_LOCATION_ASSET"] = _paths.EnginesDir;

        _process = new Process { StartInfo = psi, EnableRaisingEvents = true };
        _process.OutputDataReceived += (_, e) => { if (e.Data is not null) _logger.LogDebug("xray: {Line}", e.Data); };
        _process.ErrorDataReceived += (_, e) => { if (e.Data is not null) _logger.LogWarning("xray: {Line}", e.Data); };

        if (!_process.Start())
            throw new InvalidOperationException("XRAY FAILED TO START");

        _process.BeginOutputReadLine();
        _process.BeginErrorReadLine();

        await WaitForTunAdapterAsync(XrayWindowsConfigAdapter.TunAdapterName, ct);
    }

    private async Task WaitForTunAdapterAsync(string adapterName, CancellationToken ct)
    {
        var deadline = DateTime.UtcNow.AddSeconds(15);
        while (DateTime.UtcNow < deadline)
        {
            ct.ThrowIfCancellationRequested();
            if (_process is { HasExited: true })
                throw new InvalidOperationException($"XRAY EXITED (code {_process.ExitCode})");

            foreach (var nic in NetworkInterface.GetAllNetworkInterfaces())
            {
                if (!string.Equals(nic.Name, adapterName, StringComparison.OrdinalIgnoreCase)) continue;
                try
                {
                    _ = nic.GetIPProperties().GetIPv4Properties().Index;
                    _logger.LogInformation("xray TUN ready: {Adapter}", adapterName);
                    return;
                }
                catch (NetworkInformationException) { /* stack not ready */ }
            }

            await Task.Delay(150, ct);
        }
        throw new TimeoutException("XRAY TUN ADAPTER DID NOT COME UP");
    }

    public async ValueTask DisposeAsync()
    {
        if (_process is null) return;
        try
        {
            if (!_process.HasExited)
            {
                _process.Kill(entireProcessTree: true);
                await _process.WaitForExitAsync();
            }
        }
        catch (Exception ex)
        {
            _logger.LogDebug(ex, "xray stop");
        }
        finally
        {
            _process.Dispose();
            _process = null;
        }
    }
}
