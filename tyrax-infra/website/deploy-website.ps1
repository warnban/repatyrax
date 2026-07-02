#Requires -Version 5.1
<#
.SYNOPSIS
    Rsync/scp tyrax-website to the tyrax.tech VPS.

.EXAMPLE
    pwsh -File tyrax-infra\website\deploy-website.ps1
    pwsh -File tyrax-infra\website\deploy-website.ps1 -Host 147.45.108.102 -User root
#>
[CmdletBinding()]
param(
    [string]$ServerHost = "147.45.108.102",
    [string]$User = "root",
    [string]$RemotePath = "/var/www/tyrax.tech",
    [int]$Port = 22
)

$ErrorActionPreference = "Stop"
$repoRoot = Resolve-Path "$PSScriptRoot\..\.."
$website = Join-Path $repoRoot "tyrax-website"

if (-not (Test-Path (Join-Path $website "index.html"))) {
    throw "tyrax-website not found at $website"
}
if (-not (Test-Path (Join-Path $website "download\windows\TYRAX-Setup.exe"))) {
    throw "Missing download\windows\TYRAX-Setup.exe — run build\stage-website-download.ps1 first."
}

$sshTarget = "${User}@${ServerHost}"
Write-Host "Deploying $website -> ${sshTarget}:${RemotePath}" -ForegroundColor Cyan

# Prefer rsync when available (Git Bash / WSL / cwRsync).
$rsync = Get-Command rsync -ErrorAction SilentlyContinue
if ($rsync) {
    & rsync.Source @(
        "-avz", "--delete",
        "-e", "ssh -p $Port",
        "$website/",
        "${sshTarget}:${RemotePath}/"
    )
    if ($LASTEXITCODE -ne 0) { throw "rsync failed (exit $LASTEXITCODE)" }
}
else {
    Write-Host "rsync not found — using scp -r" -ForegroundColor Yellow
    ssh -p $Port $sshTarget "mkdir -p '$RemotePath'"
    scp -P $Port -r "$website\*" "${sshTarget}:${RemotePath}/"
    if ($LASTEXITCODE -ne 0) { throw "scp failed (exit $LASTEXITCODE)" }
}

Write-Host "Website deployed. Verify: https://tyrax.tech/download/windows/TYRAX-Setup.exe" -ForegroundColor Green
