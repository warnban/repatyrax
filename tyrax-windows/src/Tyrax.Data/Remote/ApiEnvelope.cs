using System.Text.Json.Serialization;

namespace Tyrax.Data.Remote;

/// <summary>
/// Standard backend envelope: <c>{"status":"ok","data":{...}}</c> or
/// <c>{"status":"error","message":"..."}</c>. Mirrors the Android <c>ApiResponse</c>.
/// </summary>
public sealed class ApiEnvelope<T>
{
    [JsonPropertyName("status")] public string Status { get; set; } = "";
    [JsonPropertyName("data")] public T? Data { get; set; }
    [JsonPropertyName("message")] public string? Message { get; set; }

    public bool IsOk => string.Equals(Status, "ok", StringComparison.OrdinalIgnoreCase);
}
