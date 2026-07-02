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
        ApiException { StatusCode: System.Net.HttpStatusCode.Forbidden } => new("DEVICE LIMIT REACHED"),
        ApiException { StatusCode: System.Net.HttpStatusCode.ServiceUnavailable } => new("NODE UNAVAILABLE"),
        ApiException { StatusCode: System.Net.HttpStatusCode.Unauthorized } => new("INVALID CREDENTIALS"),
        HttpRequestException => new("CONNECTION FAILED. RETRY."),
        TaskCanceledException => new("CONNECTION FAILED. RETRY."),
        _ => new(string.IsNullOrWhiteSpace(e.Message) ? "SYSTEM ERROR. NODE OFFLINE." : e.Message),
    };

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
