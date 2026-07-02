# TYRAX — Windows installer

Builds a single machine-wide installer that ships the UI (`TYRAX.exe`) and the
privileged service (`TyraxService.exe` → `TyraxProtocol`), registers the service
as auto-start LocalSystem, and cleans everything up on uninstall.

## Prerequisites

- .NET 8 SDK
- [Inno Setup 6](https://jrsoftware.org/isdl.php) (`iscc.exe` on `PATH`)
- Native engines fetched once: `pwsh -File engines\fetch-engines.ps1`
- (Release) An Authenticode code-signing certificate — **OV/EV strongly recommended**;
  an unsigned VPN trips SmartScreen and AV.

## 1. Publish

```powershell
pwsh -File build\publish.ps1
```

Outputs a self-contained build to `dist\` (UI + service share one runtime) with the
engines staged in `dist\engines\`. No .NET runtime is required on the target machine.

Smaller, runtime-dependent build (needs the .NET 8 Desktop Runtime on the target):

```powershell
pwsh -File build\publish.ps1 -FrameworkDependent
```

## 2. Compile the installer

```powershell
iscc installer\tyrax.iss
```

Produces `installer\Output\TYRAX-Setup-<version>.exe`.

After compile, stage both installers for the website (fixed download URLs):

```powershell
# Windows installer + Android release APK (build APK first if needed)
cd ..\tyrax-android
.\gradlew.bat assembleRelease
cd ..\tyrax-windows
pwsh -File build\stage-website-download.ps1
```

Or from Android only:

```powershell
pwsh -File tyrax-android\build\stage-release-apk.ps1
```

This copies:
- latest `TYRAX-Setup-*.exe` → `tyrax-website\download\windows\TYRAX-Setup.exe`
- `app-release.apk` → `tyrax-website\download\tyrax.apk`

## 3. Signing (release)

Sign the binaries during publish by setting the cert env vars first:

```powershell
$env:TYRAX_PFX = "C:\path\to\tyrax.pfx"
$env:TYRAX_PFX_PASS = "••••"
pwsh -File build\publish.ps1
```

Then sign the installer itself (configure a SignTool in Inno, or run manually):

```powershell
signtool sign /fd SHA256 /f $env:TYRAX_PFX /p $env:TYRAX_PFX_PASS `
  /tr http://timestamp.digicert.com /td SHA256 installer\Output\TYRAX-Setup-1.0.0.exe
```

## What the installer does

- Installs to `%ProgramFiles%\TYRAX` (requires admin).
- `sc create TyraxProtocol … start= auto` + crash-recovery, then starts it.
- Start Menu shortcut (+ optional desktop icon, + optional startup autostart).
- **Uninstall** stops `TYRAX.exe` / `xray.exe` / `tun2socks.exe`, then
  `sc stop` + `sc delete TyraxProtocol`, then removes files.
- **Upgrade / reinstall** runs the same stop sequence in `PrepareToInstall`
  *before* replacing files (fixes `clrjit.dll` / code 5 access denied when the
  UI or service is still running).

## Versioning

Keep three in sync when cutting a release:

1. `src\Tyrax.App\Tyrax.App.csproj` → `<Version>`
2. `installer\tyrax.iss` → `#define AppVersion`
3. The `version` field in the release manifest served at
   `https://api.tyrax.tech/download/windows/latest.json` (drives the in-app update banner).
