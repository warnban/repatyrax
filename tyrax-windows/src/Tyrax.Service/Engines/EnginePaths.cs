using System.IO;

namespace Tyrax.Service.Engines;

/// <summary>
/// Resolves the bundled native engines and the writable working directory.
///
/// <para>In production the installer places the engines next to the service
/// binary (or in an <c>engines\</c> subfolder). In development they live in
/// <c>tyrax-windows\engines\</c>, so we also walk parent directories looking for
/// an <c>engines</c> folder.</para>
/// </summary>
public sealed class EnginePaths
{
    public string EnginesDir { get; }
    public string WorkingDir { get; }

    public string XrayExe => Path.Combine(EnginesDir, "xray.exe");
    public string WinTunDll => Path.Combine(EnginesDir, "wintun.dll");
    public string GeoIpDat => Path.Combine(EnginesDir, "geoip.dat");
    public string GeoSiteDat => Path.Combine(EnginesDir, "geosite.dat");

    /// <summary>Path xray reads its generated config from.</summary>
    public string XrayConfig => Path.Combine(WorkingDir, "xray-config.json");

    public EnginePaths()
    {
        EnginesDir = ResolveEnginesDir()
            ?? throw new DirectoryNotFoundException(
                "ENGINES NOT FOUND. Run engines\\fetch-engines.ps1.");

        // ProgramData is writable by SYSTEM and survives across sessions.
        WorkingDir = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData),
            "TYRAX");
        Directory.CreateDirectory(WorkingDir);
    }

    private static string? ResolveEnginesDir()
    {
        var baseDir = AppContext.BaseDirectory;

        var candidates = new[]
        {
            Path.Combine(baseDir, "engines"),
            baseDir,
        };
        foreach (var c in candidates)
        {
            if (File.Exists(Path.Combine(c, "xray.exe")) &&
                File.Exists(Path.Combine(c, "wintun.dll")))
                return c;
        }

        // Dev fallback: walk up until we find an "engines" folder with xray.exe.
        var dir = new DirectoryInfo(baseDir);
        while (dir is not null)
        {
            var engines = Path.Combine(dir.FullName, "engines");
            if (File.Exists(Path.Combine(engines, "xray.exe")) &&
                File.Exists(Path.Combine(engines, "wintun.dll")))
                return engines;
            dir = dir.Parent;
        }
        return null;
    }
}
