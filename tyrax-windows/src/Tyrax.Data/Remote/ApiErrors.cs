using System.Net;
using System.Text.Json;
using Refit;
using Tyrax.Core;

namespace Tyrax.Data.Remote;

/// <summary>
/// Translates transport / HTTP failures into on-brand <see cref="TyraxException"/>
/// messages. Mirrors the Android <c>mapApiError</c> mapping.
/// </summary>
public static class ApiErrors
{
    public static TyraxException Map(Exception e) => e switch
    {
        TyraxException tex => tex,
        ApiException api => MapApi(api),
        HttpRequestException => new("CONNECTION FAILED. RETRY."),
        TaskCanceledException => new("CONNECTION FAILED. RETRY."),
        _ => new(string.IsNullOrWhiteSpace(e.Message) ? "SYSTEM ERROR. NODE OFFLINE." : e.Message),
    };

    private static TyraxException MapApi(ApiException api)
    {
        var bodyMessage = TryReadMessage(api.Content);
        if (!string.IsNullOrWhiteSpace(bodyMessage))
            return new TyraxException(bodyMessage);

        return api.StatusCode switch
        {
            HttpStatusCode.Forbidden => new("DEVICE LIMIT REACHED"),
            HttpStatusCode.ServiceUnavailable => new("NODE UNAVAILABLE"),
            HttpStatusCode.Unauthorized => new("INVALID CREDENTIALS"),
            HttpStatusCode.BadRequest => new("INVALID CREDENTIALS"),
            HttpStatusCode.Conflict => new("IDENTITY ALREADY EXISTS"),
            HttpStatusCode.TooManyRequests => new("RATE LIMIT EXCEEDED. STAND DOWN."),
            _ => new(string.IsNullOrWhiteSpace(api.Message) ? "SYSTEM ERROR. NODE OFFLINE." : api.Message),
        };
    }

    private static string? TryReadMessage(string? json)
    {
        if (string.IsNullOrWhiteSpace(json)) return null;
        try
        {
            using var doc = JsonDocument.Parse(json);
            if (doc.RootElement.TryGetProperty("message", out var msg))
            {
                var text = msg.GetString();
                return string.IsNullOrWhiteSpace(text) ? null : text;
            }
        }
        catch (JsonException) { }
        return null;
    }

    /// <summary>Runs <paramref name="call"/>, unwrapping the envelope or throwing branded.</summary>
    public static async Task<T> UnwrapAsync<T>(Func<Task<ApiEnvelope<T>>> call, string emptyMessage)
    {
        try
        {
            var env = await call();
            if (env.IsOk && env.Data is not null) return env.Data;
            throw new TyraxException(env.Message ?? emptyMessage);
        }
        catch (Exception e)
        {
            throw Map(e);
        }
    }
}
