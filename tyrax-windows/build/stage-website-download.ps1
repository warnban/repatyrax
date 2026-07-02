#Requires -Version 5.1
<#
.SYNOPSIS
    Copies the latest Windows installer and Android release APK into tyrax-website/download/
    for tyrax.tech fixed download URLs.

.EXAMPLE
    pwsh -File build\stage-website-download.ps1
    pwsh -File build\stage-website-download.ps1 -SkipWindows
    pwsh -File build\stage-website-download.ps1 -ApkPath ..\..\tyrax-android\app\build\outputs\apk\release\app-release.apk
#>
[CmdletBinding()]
param(
    [string]$ApkPath = "",
    [switch]$SkipWindows,
    [switch]$SkipAndroid
)

$ErrorActionPreference = "Stop"
$root = Resolve-Path "$PSScriptRoot\.."
$repoRoot = Resolve-Path "$root\.."
$websiteDl = Join-Path $repoRoot "tyrax-website\download"
$releaseApkDefault = Join-Path $repoRoot "tyrax-android\app\build\outputs\apk\release\app-release.apk"

if (-not $SkipWindows) {
    $installerOut = Join-Path $root "installer\Output"
    $setup = Get-ChildItem (Join-Path $installerOut "TYRAX-Setup-*.exe") -ErrorAction Stop |
        Sort-Object { [version]($_.BaseName -replace '^TYRAX-Setup-', '') } -Descending |
        Select-Object -First 1

    if (-not $setup) { throw "No TYRAX-Setup-*.exe in $installerOut. Run iscc installer\tyrax.iss first." }

    $winDestDir = Join-Path $websiteDl "windows"
    New-Item -ItemType Directory -Force -Path $winDestDir | Out-Null
    $winDest = Join-Path $winDestDir "TYRAX-Setup.exe"
    Copy-Item -LiteralPath $setup.FullName -Destination $winDest -Force
    $version = ($setup.BaseName -replace '^TYRAX-Setup-', '')
    $manifest = @{
        version = $version
        url     = "https://tyrax.tech/download/windows/TYRAX-Setup.exe"
    } | ConvertTo-Json -Compress
    Set-Content -Path (Join-Path $winDestDir "latest.json") -Value $manifest -Encoding UTF8
    Write-Host "Windows: $($setup.Name) -> tyrax-website\download\windows\TYRAX-Setup.exe ($version)" -ForegroundColor Green
}

if (-not $SkipAndroid) {
    if ([string]::IsNullOrWhiteSpace($ApkPath)) {
        $ApkPath = $releaseApkDefault
    }
    if (-not (Test-Path $ApkPath)) {
        throw "Release APK not found at $ApkPath. Run: cd tyrax-android && .\gradlew.bat assembleRelease"
    }
    New-Item -ItemType Directory -Force -Path $websiteDl | Out-Null
    Copy-Item -LiteralPath $ApkPath -Destination (Join-Path $websiteDl "tyrax.apk") -Force
    Write-Host "Android: $ApkPath -> tyrax-website\download\tyrax.apk (release)" -ForegroundColor Green
}

Write-Host "Deploy tyrax-website/ (including download/) to tyrax.tech." -ForegroundColor Cyan
