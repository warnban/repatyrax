namespace Tyrax.Service.Engines;

/// <summary>
/// Fixed local parameters for the WinTun adapter and helper endpoints. Kept in
/// one place so xray, the network configurator and teardown all agree on the
/// adapter name, addressing and stats source.
/// </summary>
public static class TunnelConstants
{
    /// <summary>WinTun adapter name; xray TUN inbound creates it on Windows.</summary>
    public const string AdapterName = "TYRAX";

    /// <summary>Address assigned to the TUN adapter (a private /24 unlikely to clash).</summary>
    public const string TunAddress = "10.7.0.2";
    public const string TunMask = "255.255.255.0";

    /// <summary>On-link gateway inside the TUN subnet that default routes point at.</summary>
    public const string TunGateway = "10.7.0.1";

    /// <summary>
    /// TUN device MTU passed to xray's TUN inbound. 1400 leaves headroom under
    /// 1500 on the physical path and avoids silent receive-path drops on WinTun.
    /// </summary>
    public const int TunMtu = 1400;

    /// <summary>DNS pushed onto the TUN adapter so lookups exit through the PROTOCOL.</summary>
    public const string PrimaryDns = "1.1.1.1";
    public const string SecondaryDns = "1.0.0.1";
}
