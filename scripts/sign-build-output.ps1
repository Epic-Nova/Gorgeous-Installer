#!/usr/bin/env pwsh
# Compatibility entrypoint; Windows helper logic lives in scripts/windows.
param([Parameter(Mandatory = $true)][string]$CertPath, [string]$CertPassword, [string]$ExePath = "build/gorgeous-installer.exe", [string]$TimestampUrl = "http://timestamp.digicert.com")
$ErrorActionPreference = "Stop"
& (Join-Path $PSScriptRoot "windows/sign-build-output.ps1") @PSBoundParameters
exit $LASTEXITCODE
