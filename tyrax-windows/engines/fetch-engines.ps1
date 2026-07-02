#Requires -Version 5.1
<#
.SYNOPSIS
    Fetches the native engines the TYRAX Windows client bundles: Xray-core,
    wintun.dll and the geoip/geosite databases.

.DESCRIPTION
    The binaries are intentionally NOT committed to git (see .gitignore). Run this
    once after cloning to populate tyrax-windows/engines/. Re-run to update.
#>
[CmdletBinding()]
param(
    # Pin versions for reproducible builds; empty string = latest release.
    [string]$XrayVersion = ""
)

$ErrorActionPreference = "Stop"
$dir = $PSScriptRoot
Write-Host "Populating engines in $dir" -ForegroundColor Cyan

function Get-Release([string]$repo, [string]$version, [string]$assetPattern) {
    if ([string]::IsNullOrEmpty($version)) {
        return "https://github.com/$repo/releases/latest/download/$assetPattern"
    }
    return "https://github.com/$repo/releases/download/$version/$assetPattern"
}

# ── Xray-core (ships xray.exe + geoip.dat + geosite.dat in the zip) ──────────
$xrayUrl = Get-Release "XTLS/Xray-core" $XrayVersion "Xray-windows-64.zip"
Write-Host "Xray  : $xrayUrl"
Invoke-WebRequest -Uri $xrayUrl -OutFile "$dir\xray.zip" -UseBasicParsing
Expand-Archive -Path "$dir\xray.zip" -DestinationPath "$dir\_xray" -Force
Copy-Item "$dir\_xray\xray.exe"     "$dir\xray.exe"     -Force
Copy-Item "$dir\_xray\geoip.dat"    "$dir\geoip.dat"    -Force
Copy-Item "$dir\_xray\geosite.dat"  "$dir\geosite.dat"  -Force
Remove-Item "$dir\_xray" -Recurse -Force
Remove-Item "$dir\xray.zip" -Force

# ── wintun.dll (from the WireGuard project; required by xray TUN inbound) ──────
Write-Host "wintun: https://www.wintun.net/builds/wintun-0.14.1.zip"
Invoke-WebRequest -Uri "https://www.wintun.net/builds/wintun-0.14.1.zip" -OutFile "$dir\wintun.zip" -UseBasicParsing
Expand-Archive -Path "$dir\wintun.zip" -DestinationPath "$dir\_wintun" -Force
Copy-Item "$dir\_wintun\wintun\bin\amd64\wintun.dll" "$dir\wintun.dll" -Force
Remove-Item "$dir\_wintun" -Recurse -Force
Remove-Item "$dir\wintun.zip" -Force

Write-Host "Done. Engines:" -ForegroundColor Green
Get-ChildItem $dir -Exclude *.ps1,*.md | Format-Table Name,Length -AutoSize
