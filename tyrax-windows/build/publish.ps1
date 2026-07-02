#Requires -Version 5.1
<#
.SYNOPSIS
    Publishes the TYRAX Windows client (UI + privileged service) into a single
    self-contained folder and stages the native engines beside the service, ready
    for the Inno Setup installer (installer\tyrax.iss).

.DESCRIPTION
    Both TYRAX.exe (WPF UI) and TyraxService.exe (worker) are published
    self-contained for win-x64 into the SAME output folder: they target the exact
    same runtime, so the shared framework files overlap byte-for-byte and the user
    needs no .NET install. Engines (xray/wintun + geo data) are copied
    into <out>\engines so EnginePaths finds them next to the service binary.

    Set TYRAX_PFX (+ TYRAX_PFX_PASS) to Authenticode-sign every shipped binary.
    For release, use an OV/EV certificate - an unsigned VPN tripping SmartScreen
    and AV is the fastest way to lose trust.

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File build\publish.ps1
    powershell -ExecutionPolicy Bypass -File build\publish.ps1 -Configuration Release -FrameworkDependent
#>
[CmdletBinding()]
param(
    [string]$Configuration = "Release",
    [string]$Runtime = "win-x64",
    # Defaults to <repo>\dist; computed below since $PSScriptRoot isn't reliable in param defaults on Windows PowerShell.
    [string]$Out,
    # Publish smaller framework-dependent builds instead (needs .NET 8 Desktop Runtime on the target).
    [switch]$FrameworkDependent
)

$ErrorActionPreference = "Stop"
$root = Resolve-Path "$PSScriptRoot\.."
if ([string]::IsNullOrWhiteSpace($Out)) { $Out = Join-Path $root "dist" }
$appProj = Join-Path $root "src\Tyrax.App\Tyrax.App.csproj"
$svcProj = Join-Path $root "src\Tyrax.Service\Tyrax.Service.csproj"
$enginesSrc = Join-Path $root "engines"

$selfContained = -not $FrameworkDependent

Write-Host "TYRAX publish -> $Out ($Configuration/$Runtime, self-contained=$selfContained)" -ForegroundColor Cyan

# Guard: engines must be present (they are not committed).
$required = @("xray.exe", "wintun.dll", "geoip.dat", "geosite.dat")
foreach ($f in $required) {
    if (-not (Test-Path (Join-Path $enginesSrc $f))) {
        throw "Missing engine '$f'. Run engines\fetch-engines.ps1 first."
    }
}

# Clean output.
if (Test-Path $Out) { Remove-Item $Out -Recurse -Force }
New-Item -ItemType Directory -Path $Out | Out-Null

function Publish-Project([string]$proj, [string]$name) {
    Write-Host "Publishing $name..." -ForegroundColor DarkCyan
    dotnet publish $proj `
        -c $Configuration `
        -r $Runtime `
        --self-contained $selfContained `
        -p:PublishSingleFile=false `
        -o $Out
    if ($LASTEXITCODE -ne 0) { throw "publish failed for $name" }
}

# Service first, UI second - shared runtime files overwrite identically.
Publish-Project $svcProj "TyraxService"
Publish-Project $appProj "TYRAX (UI)"

# Stage engines next to the service binary.
$enginesOut = Join-Path $Out "engines"
New-Item -ItemType Directory -Path $enginesOut -Force | Out-Null
foreach ($f in $required) {
    Copy-Item (Join-Path $enginesSrc $f) (Join-Path $enginesOut $f) -Force
}

# Optional Authenticode signing.
if ($env:TYRAX_PFX) {
    $signtool = Get-Command signtool.exe -ErrorAction SilentlyContinue
    if (-not $signtool) { throw "TYRAX_PFX set but signtool.exe not on PATH (install the Windows SDK)." }

    $toSign = @(
        (Join-Path $Out "TYRAX.exe"),
        (Join-Path $Out "TyraxService.exe"),
        (Join-Path $enginesOut "xray.exe")
    )
    $passArgs = @()
    if ($env:TYRAX_PFX_PASS) { $passArgs = @("/p", $env:TYRAX_PFX_PASS) }

    foreach ($bin in $toSign) {
        Write-Host "Signing $bin" -ForegroundColor Yellow
        & $signtool.Path sign /fd SHA256 /f $env:TYRAX_PFX @passArgs `
            /tr http://timestamp.digicert.com /td SHA256 $bin
        if ($LASTEXITCODE -ne 0) { throw "signing failed for $bin" }
    }
}
else {
    Write-Host "TYRAX_PFX not set - skipping code signing (dev build)." -ForegroundColor DarkYellow
}

Write-Host "Done. Output: $Out" -ForegroundColor Green
Write-Host "Next: compile the installer with installer\tyrax.iss (see installer\README.md)." -ForegroundColor Green
