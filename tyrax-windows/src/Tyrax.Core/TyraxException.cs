namespace Tyrax.Core;

/// <summary>
/// Carries an on-brand, user-facing message ("NODE UNAVAILABLE", "DEVICE LIMIT
/// REACHED", …). Repositories translate transport/HTTP failures into this so the
/// UI never surfaces raw stack traces or "Error 500".
/// </summary>
public sealed class TyraxException : Exception
{
    public TyraxException(string message) : base(message) { }
    public TyraxException(string message, Exception inner) : base(message, inner) { }
}
