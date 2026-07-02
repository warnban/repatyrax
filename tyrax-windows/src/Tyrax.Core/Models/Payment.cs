namespace Tyrax.Core.Models;

/// <summary>Result of <c>POST /payment/create</c>: an order + a hosted payment URL.</summary>
public sealed record PaymentResult
{
    public required string OrderId { get; init; }
    public required string PaymentUrl { get; init; }
    public double AmountRub { get; init; }
}

/// <summary>Poll result of <c>GET /payment/status/{orderId}</c>. <see cref="OrderStatus"/> is <c>PAID</c> when done.</summary>
public sealed record PaymentStatus
{
    public required string OrderStatus { get; init; }
    public required string Tier { get; init; }
}
