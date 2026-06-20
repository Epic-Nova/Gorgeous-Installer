#!/usr/bin/env pwsh
# Signs an existing build output using an Authenticode code-signing certificate.

param(
    [Parameter(Mandatory = $true)]
    [string]$CertPath,
    [string]$CertPassword,
    [string]$ExePath = ".\\build\\gorgeous-installer.exe",
    [string]$TimestampUrl = "http://timestamp.digicert.com"
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Push-Location $scriptDir

try {
    if (-not (Test-Path $ExePath)) {
        Write-Host "Build output not found: $ExePath" -ForegroundColor Red
        exit 1
    }

    if (-not (Test-Path $CertPath)) {
        Write-Host "Certificate file not found: $CertPath" -ForegroundColor Red
        exit 1
    }

    $signtool = Get-Command signtool.exe -ErrorAction SilentlyContinue
    if (-not $signtool) {
        Write-Host "signtool.exe not found. Install the Windows SDK to enable signing." -ForegroundColor Red
        exit 1
    }

    $resolvedExe = (Resolve-Path $ExePath).Path
    $resolvedCert = (Resolve-Path $CertPath).Path

    $signArgs = @("sign", "/fd", "SHA256", "/f", $resolvedCert)
    if (-not [string]::IsNullOrWhiteSpace($CertPassword)) {
        $signArgs += @("/p", $CertPassword)
    }
    $signArgs += @("/tr", $TimestampUrl, "/td", "SHA256", $resolvedExe)

    Write-Host "Signing executable: $resolvedExe" -ForegroundColor Yellow
    & $signtool.Source @signArgs
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Code signing failed" -ForegroundColor Red
        exit 1
    }

    Write-Host "Code signing successful" -ForegroundColor Green
} finally {
    Pop-Location
}
