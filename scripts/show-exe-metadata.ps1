#!/usr/bin/env pwsh
# Shows Windows version metadata, signature status, and embedded build info for the installer executable.

param(
    [string]$ExePath = ".\\build\\gorgeous-installer.exe",
    [string]$BuildMetadataPath = ".\\build\\gorgeous-installer.build.json"
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Push-Location $scriptDir

try {
    if (-not (Test-Path $ExePath)) {
        Write-Host "Executable not found: $ExePath" -ForegroundColor Red
        exit 1
    }

    $resolvedExe = (Resolve-Path $ExePath).Path
    $item = Get-Item $resolvedExe

    Write-Host "Executable: $resolvedExe" -ForegroundColor Cyan
    Write-Host "Size MB: $([math]::Round($item.Length / 1MB, 2))" -ForegroundColor Cyan

    Write-Host "" 
    Write-Host "Version Metadata" -ForegroundColor Yellow
    $item.VersionInfo | Format-List FileDescription,ProductName,CompanyName,FileVersion,ProductVersion,Comments,PrivateBuild,SpecialBuild

    Write-Host "" 
    Write-Host "Authenticode Signature" -ForegroundColor Yellow
    $signature = Get-AuthenticodeSignature -FilePath $resolvedExe
    $signature | Format-List Status,StatusMessage,SignerCertificate,TimeStamperCertificate

    Write-Host "" 
    Write-Host "Embedded Build Info" -ForegroundColor Yellow
    $embeddedInfo = (& $resolvedExe -version-info 2>$null | Out-String).Trim()
    if ([string]::IsNullOrWhiteSpace($embeddedInfo)) {
        Write-Host "No console output available from this Windows GUI executable." -ForegroundColor Yellow
        Write-Host "Use the build metadata JSON below for commit/build-time information." -ForegroundColor Yellow
    } else {
        Write-Host $embeddedInfo
    }

    if (Test-Path $BuildMetadataPath) {
        Write-Host "" 
        Write-Host "Build Metadata JSON" -ForegroundColor Yellow
        Get-Content $BuildMetadataPath -Raw
    }
} finally {
    Pop-Location
}
