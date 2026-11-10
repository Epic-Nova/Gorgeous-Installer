#!/usr/bin/env pwsh
# Sign only when explicitly invoked with a certificate; never install tooling.
param(
    [Parameter(Mandatory = $true)][string]$CertPath,
    [string]$CertPassword,
    [string]$ExePath = "build/gorgeous-installer.exe",
    [string]$TimestampUrl = "http://timestamp.digicert.com"
)
$ErrorActionPreference = "Stop"
$repositoryRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
if (-not [IO.Path]::IsPathRooted($ExePath)) { $ExePath = Join-Path $repositoryRoot $ExePath }
if (-not (Test-Path -LiteralPath $ExePath)) { throw "Build output not found: $ExePath" }
if (-not (Test-Path -LiteralPath $CertPath)) { throw "Certificate file not found: $CertPath" }
$signtool = Get-Command signtool.exe -ErrorAction SilentlyContinue
if (-not $signtool) { throw "signtool.exe not found. Install the Windows SDK explicitly to enable signing." }
$resolvedExe = (Resolve-Path -LiteralPath $ExePath).Path
$resolvedCert = (Resolve-Path -LiteralPath $CertPath).Path
$signArgs = @("sign", "/fd", "SHA256", "/f", $resolvedCert)
if (-not [string]::IsNullOrWhiteSpace($CertPassword)) { $signArgs += @("/p", $CertPassword) }
$signArgs += @("/tr", $TimestampUrl, "/td", "SHA256", $resolvedExe)
Write-Host "Signing executable: $resolvedExe"
& $signtool.Source @signArgs
if ($LASTEXITCODE -ne 0) { throw "Code signing failed with exit code $LASTEXITCODE" }
Write-Host "Code signing successful"
