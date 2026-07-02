#Requires -Version 5.1
<#
.SYNOPSIS
    Pull latest code and rebuild the API on the backend VPS.

.EXAMPLE
    pwsh -File tyrax-infra\backend\deploy-backend-update.ps1
    pwsh -File tyrax-infra\backend\deploy-backend-update.ps1 -Host 5.129.195.144
#>
[CmdletBinding()]
param(
    [string]$ServerHost = "5.129.195.144",
    [string]$User = "root",
    [string]$RepoDir = "/opt/tyrax",
    [int]$Port = 22
)

$ErrorActionPreference = "Stop"
$sshTarget = "${User}@${ServerHost}"
$remote = @"
set -e
cd '$RepoDir'
git fetch --all
git pull --ff-only
cd tyrax-backend
docker compose up -d --build
docker compose ps
curl -fsS http://127.0.0.1:8080/health
"@

Write-Host "Updating backend on $sshTarget ($RepoDir)..." -ForegroundColor Cyan
ssh -p $Port $sshTarget $remote
Write-Host "Backend updated. Verify: https://api.tyrax.tech/health" -ForegroundColor Green
