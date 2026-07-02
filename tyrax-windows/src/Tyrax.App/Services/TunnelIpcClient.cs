using System.IO;
using System.IO.Pipes;
using System.Text;
using System.Text.Json;
using Tyrax.Ipc;

namespace Tyrax.App.Services;

/// <summary>
/// Client half of the named-pipe bridge to <c>TyraxService</c>. Connects, streams
/// <see cref="IpcStatus"/> pushes via <see cref="StatusReceived"/>, and sends
/// <see cref="IpcCommand"/> frames. Reconnects are the caller's responsibility
/// (Phase 3 adds an auto-reconnect wrapper).
/// </summary>
public sealed class TunnelIpcClient : IAsyncDisposable
{
    private NamedPipeClientStream? _pipe;
    private StreamReader? _reader;
    private StreamWriter? _writer;
    private CancellationTokenSource? _cts;

    /// <summary>Fired on the reader task when the service pushes a status frame.</summary>
    public event Action<IpcStatus>? StatusReceived;

    /// <summary>Fired when the pipe drops, so the UI can show OUTSIDE SYSTEM.</summary>
    public event Action? Disconnected;

    public bool IsConnected => _pipe?.IsConnected == true;

    public async Task ConnectAsync(CancellationToken ct = default)
    {
        await CleanupAsync(); // drop any stale connection before re-dialing

        _pipe = new NamedPipeClientStream(".", IpcProtocol.PipeName,
            PipeDirection.InOut, PipeOptions.Asynchronous);
        await _pipe.ConnectAsync(3000, ct);

        _reader = new StreamReader(_pipe, Encoding.UTF8, false, 4096, leaveOpen: true);
        _writer = new StreamWriter(_pipe, new UTF8Encoding(false), 4096, leaveOpen: true)
        {
            AutoFlush = true,
        };

        _cts = CancellationTokenSource.CreateLinkedTokenSource(ct);
        _ = Task.Run(() => ReadLoopAsync(_cts.Token));
    }

    private async Task ReadLoopAsync(CancellationToken ct)
    {
        try
        {
            string? line;
            while (!ct.IsCancellationRequested && _reader is not null &&
                   (line = await _reader.ReadLineAsync(ct)) is not null)
            {
                if (string.IsNullOrWhiteSpace(line)) continue;
                IpcStatus? status;
                try
                {
                    status = JsonSerializer.Deserialize<IpcStatus>(line, IpcProtocol.Json);
                }
                catch (JsonException)
                {
                    continue;
                }
                if (status is not null) StatusReceived?.Invoke(status);
            }
        }
        catch (Exception) { /* pipe closed */ }
        finally
        {
            Disconnected?.Invoke();
        }
    }

    public async Task SendAsync(IpcCommand command, CancellationToken ct = default)
    {
        if (_writer is null) throw new InvalidOperationException("PIPE NOT CONNECTED");
        var json = JsonSerializer.Serialize(command, IpcProtocol.Json);
        await _writer.WriteLineAsync(json.AsMemory(), ct);
    }

    private async Task CleanupAsync()
    {
        _cts?.Cancel();
        _cts?.Dispose();
        _cts = null;
        if (_writer is not null) { await _writer.DisposeAsync(); _writer = null; }
        _reader?.Dispose();
        _reader = null;
        if (_pipe is not null) { await _pipe.DisposeAsync(); _pipe = null; }
    }

    public async ValueTask DisposeAsync() => await CleanupAsync();
}
