#!/usr/bin/env pwsh
# Compatibility entrypoint; Windows helper logic lives in scripts/windows.
param([string]$ConfigPath = "", [switch]$Strict, [switch]$KeepBrokenManifests)
$ErrorActionPreference = "Stop"
& (Join-Path $PSScriptRoot "windows/test-sha-comparison.ps1") @PSBoundParameters
exit $LASTEXITCODE
