#!/usr/bin/env pwsh
# Compatibility entrypoint; Windows helper logic lives in scripts/windows.
param([string]$ExePath = "build/gorgeous-installer.exe", [string]$BuildMetadataPath = "build/gorgeous-installer.build.json")
$ErrorActionPreference = "Stop"
& (Join-Path $PSScriptRoot "windows/show-exe-metadata.ps1") @PSBoundParameters
exit $LASTEXITCODE
