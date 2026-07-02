using System.IO.Pipes;
using System.Security.AccessControl;
using System.Security.Principal;
using System.Text;
using System.Text.Json;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using Tyrax.Ipc;

namespace Tyrax.Service;

/// <summary>
/// Named-pipe server: one client (the WPF UI) at a time. Reads newline-delimited
/// <see cref="IpcCommand"/> JSON, drives the <see cref="TunnelSupervisor"/>, and
/// pushes <see cref="IpcStatus"/> back both on demand and whenever state changes.
/// </summary>
public sealed class IpcServer : BackgroundService
{
    private readonly TunnelSupervisor _supervisor;
    private readonly ILogger<IpcServer> _logger;

    public IpcServer(TunnelSupervisor supervisor, ILogger<IpcServer> logger)
    {
        _supervisor = supervisor;
        _logger = logger;
    }

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        _logger.LogInformation("TYRAX SERVICE ONLINE, pipe {Pipe}", IpcProtocol.PipeName);

        while (!stoppingToken.IsCancellationRequested)
        {
            try
            {
                await using var server = CreatePipe();
                await server.WaitForConnectionAsync(stoppingToken);
                await HandleClientAsync(server, stoppingToken);
            }
            catch (OperationCanceledException)
            {
                break;
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "IPC session failed");
                await Task.Delay(500, stoppingToken);
            }
        }
    }

    private async Task HandleClientAsync(NamedPipeServerStream server, CancellationToken ct)
    {
        using var reader = new StreamReader(server, Encoding.UTF8, false, 4096, leaveOpen: true);
        await using var writer = new StreamWriter(server, new UTF8Encoding(false), 4096, leaveOpen: true)
        {
            AutoFlush = true,
        };

        void OnStatus(IpcStatus s) => TryWrite(writer, s);
        _supervisor.StatusChanged += OnStatus;
        try
        {
            // Greet the freshly-connected UI with the current snapshot.
            await WriteAsync(writer, _supervisor.Snapshot());

            string? line;
            while (!ct.IsCancellationRequested && (line = await reader.ReadLineAsync(ct)) is not null)
            {
                if (string.IsNullOrWhiteSpace(line)) continue;

                IpcCommand? cmd;
                try
                {
                    cmd = JsonSerializer.Deserialize<IpcCommand>(line, IpcProtocol.Json);
                }
                catch (JsonException)
                {
                    continue; // ignore malformed frames
                }
                if (cmd is null) continue;

                var status = cmd.Kind switch
                {
                    IpcCommandKind.Connect => await _supervisor.ConnectAsync(cmd, ct),
                    IpcCommandKind.Disconnect => await _supervisor.DisconnectAsync(ct),
                    _ => _supervisor.Snapshot(),
                };
                await WriteAsync(writer, status);
            }
        }
        finally
        {
            _supervisor.StatusChanged -= OnStatus;
        }
    }

    private static async Task WriteAsync(StreamWriter writer, IpcStatus status)
    {
        var json = JsonSerializer.Serialize(status, IpcProtocol.Json);
        await writer.WriteLineAsync(json);
    }

    private void TryWrite(StreamWriter writer, IpcStatus status)
    {
        try
        {
            writer.WriteLine(JsonSerializer.Serialize(status, IpcProtocol.Json));
        }
        catch (Exception ex)
        {
            _logger.LogDebug(ex, "status push dropped");
        }
    }

    /// <summary>
    /// Creates the pipe with an ACL that lets any authenticated user connect
    /// (the UI runs unprivileged) while only SYSTEM/Administrators can create the
    /// server instance — a non-elevated process cannot squat the name.
    /// </summary>
    private static NamedPipeServerStream CreatePipe()
    {
        var security = new PipeSecurity();
        security.AddAccessRule(new PipeAccessRule(
            new SecurityIdentifier(WellKnownSidType.AuthenticatedUserSid, null),
            PipeAccessRights.ReadWrite,
            AccessControlType.Allow));
        security.AddAccessRule(new PipeAccessRule(
            new SecurityIdentifier(WellKnownSidType.LocalSystemSid, null),
            PipeAccessRights.FullControl,
            AccessControlType.Allow));

        return NamedPipeServerStreamAcl.Create(
            IpcProtocol.PipeName,
            PipeDirection.InOut,
            maxNumberOfServerInstances: 1,
            PipeTransmissionMode.Byte,
            PipeOptions.Asynchronous,
            inBufferSize: 0,
            outBufferSize: 0,
            security);
    }
}
