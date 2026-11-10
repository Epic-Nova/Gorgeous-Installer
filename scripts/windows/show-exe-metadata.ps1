#!/usr/bin/env pwsh
# Inspect the repository build; relative asset paths are relative to the repository.
param(
    [string]$ExePath = "build/gorgeous-installer.exe",
    [string]$BuildMetadataPath = "build/gorgeous-installer.build.json"
)
$ErrorActionPreference = "Stop"
$repositoryRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
if (-not [IO.Path]::IsPathRooted($ExePath)) { $ExePath = Join-Path $repositoryRoot $ExePath }
if (-not [IO.Path]::IsPathRooted($BuildMetadataPath)) { $BuildMetadataPath = Join-Path $repositoryRoot $BuildMetadataPath }
if (-not (Test-Path -LiteralPath $ExePath)) { throw "Executable not found: $ExePath" }
$resolvedExe = (Resolve-Path -LiteralPath $ExePath).Path
$item = Get-Item -LiteralPath $resolvedExe
Write-Host "Executable: $resolvedExe"
Write-Host "Size MB: $([math]::Round($item.Length / 1MB, 2))"
Write-Host "Version Metadata"
$item.VersionInfo | Format-List FileDescription,ProductName,CompanyName,FileVersion,ProductVersion,Comments,PrivateBuild,SpecialBuild
Write-Host "Authenticode Signature"
Get-AuthenticodeSignature -FilePath $resolvedExe | Format-List Status,StatusMessage,SignerCertificate,TimeStamperCertificate
Write-Host "Embedded Build Info"
$embeddedInfo = (& $resolvedExe -version-info 2>$null | Out-String).Trim()
if ($LASTEXITCODE -ne 0) { throw "Executable build-info query failed with exit code $LASTEXITCODE" }
if ([string]::IsNullOrWhiteSpace($embeddedInfo)) {
    Write-Host "No console output available; inspect the build metadata JSON."
} else { Write-Host $embeddedInfo }
if (Test-Path -LiteralPath $BuildMetadataPath) {
    Write-Host "Build Metadata JSON"
    Get-Content -LiteralPath $BuildMetadataPath -Raw
}
