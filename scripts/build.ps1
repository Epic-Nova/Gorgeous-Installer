#!/usr/bin/env pwsh
# Compatibility entrypoint; the repository root owns the build implementation.
param(
    [switch]$Clean, [switch]$Run, [switch]$UseUPX, [switch]$SkipUPX,
    [switch]$StrictPackSHA, [switch]$TestSHAComparison, [switch]$Sign,
    [string]$CertPath, [string]$CertPassword,
    [string]$TimestampUrl = "http://timestamp.digicert.com"
)
$ErrorActionPreference = "Stop"
& (Join-Path (Split-Path -Parent $PSScriptRoot) "build.ps1") @PSBoundParameters
exit $LASTEXITCODE
