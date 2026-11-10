#!/usr/bin/env pwsh
# Confirm valid and intentionally mismatched manifests through the real Go validator.
param([string]$ConfigPath = "", [switch]$Strict, [switch]$KeepBrokenManifests)
$ErrorActionPreference = "Stop"
$repositoryRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
if ([string]::IsNullOrWhiteSpace($ConfigPath)) { $ConfigPath = Join-Path $repositoryRoot "config.json" }
$ConfigPath = [IO.Path]::GetFullPath($ConfigPath)
$configRoot = Split-Path -Parent $ConfigPath

function Resolve-AssetPath([string]$Value) {
    if ([IO.Path]::IsPathRooted($Value)) { return [IO.Path]::GetFullPath($Value) }
    return [IO.Path]::GetFullPath((Join-Path $configRoot $Value))
}
function Test-IsSHAControlFile([string]$Name) {
    $lower = $Name.ToLowerInvariant()
    return $lower.EndsWith(".sha") -or $lower.EndsWith(".sha256") -or $lower.EndsWith(".sha512") -or
        $lower -eq "sha256.txt" -or $lower -eq "checksums.txt"
}
function Get-PackManifestLines([string]$PackRoot) {
    $root = [IO.Path]::GetFullPath($PackRoot).TrimEnd([char[]]'\/') + [IO.Path]::DirectorySeparatorChar
    foreach ($file in (Get-ChildItem -LiteralPath $PackRoot -Recurse -File | Sort-Object FullName)) {
        if (Test-IsSHAControlFile $file.Name) { continue }
        $relative = $file.FullName.Substring($root.Length).Replace([char]92, [char]47)
        $hash = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        "$hash  $relative"
    }
}

$utf8 = New-Object System.Text.UTF8Encoding($false)
$backups = @()
$tested = New-Object System.Collections.Generic.List[string]
$runnerDirectory = $null
Push-Location $repositoryRoot
try {
    if (-not (Test-Path -LiteralPath $ConfigPath)) { throw "Config file not found: $ConfigPath" }
    $config = Get-Content -LiteralPath $ConfigPath -Raw -Encoding UTF8 | ConvertFrom-Json
    if (-not $config.availableVersions -or $config.availableVersions.Count -eq 0) { throw "No availableVersions were found in $ConfigPath" }
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw "Go is required; install the toolchain explicitly." }

    # The uniquely owned probe avoids changing config.json or embedded inputs.
    # Compilation failures cannot masquerade as confirmed SHA mismatches.
    $runnerDirectory = Join-Path $repositoryRoot ("scripts/.sha-comparison-" + [Guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $runnerDirectory | Out-Null
    $probeSource = @'
package main
import (
    "fmt"
    "os"
    "gorgeous-installer/internal/config"
    "gorgeous-installer/internal/installer"
)
func main() {
    if len(os.Args)!=5 { os.Exit(2) }
    pack:=config.PackVersion{Version:os.Args[2],Path:os.Args[3],SHAFile:os.Args[4]}
    report,err:=installer.ValidatePackSHA(&pack,os.Args[4])
    if err!=nil { fmt.Fprintln(os.Stderr,err);os.Exit(2) }
    if os.Args[1]=="valid" && report.TotalEntries>0 && report.IsValid() {
        fmt.Println("SHA_PROBE_VALID");return
    }
    if os.Args[1]=="mismatch" && report.TotalEntries>0 && len(report.Mismatches)>0 && len(report.MissingFiles)==0 {
        fmt.Println("SHA_PROBE_EXPECTED_MISMATCH");return
    }
    fmt.Fprintf(os.Stderr,"Unexpected SHA result: %+v\n",report);os.Exit(1)
}
'@
    $probeFile = Join-Path $runnerDirectory "main.go"
    $probeExe = Join-Path $runnerDirectory "sha-probe.exe"
    [IO.File]::WriteAllText($probeFile, $probeSource, $utf8)
    & go build -o $probeExe $probeFile
    if ($LASTEXITCODE -ne 0) { throw "SHA probe compilation failed; no mismatch test passed." }

    foreach ($entry in $config.availableVersions) {
        $version = [string]$entry.version
        $packPath = [string]$entry.path
        if ([string]::IsNullOrWhiteSpace($version) -or [string]::IsNullOrWhiteSpace($packPath)) {
            if ($Strict) { throw "Version/path is empty" }
            Write-Host "Skipping entry with empty version/path"
            continue
        }
        $normalized = $packPath.Replace([char]92, [char]47).ToLowerInvariant()
        if (-not ($normalized -eq "content" -or $normalized.EndsWith("/content") -or $normalized.Contains("/content/"))) {
            Write-Host "Skipping $($version): not a content pack path"
            continue
        }
        $packRoot = Resolve-AssetPath $packPath
        if (-not (Test-Path -LiteralPath $packRoot)) {
            if ($Strict) { throw "Pack path not found: $packRoot" }
            Write-Host "Skipping $($version): pack path not found"
            continue
        }
        $manifest = [string]$entry.shaFile
        if ([string]::IsNullOrWhiteSpace($manifest)) {
            $parent = Split-Path -Parent $packPath
            if ([string]::IsNullOrWhiteSpace($parent)) { $manifest = "$packPath.sha256" }
            else { $manifest = Join-Path $parent "manifest.sha256" }
        }
        $manifest = Resolve-AssetPath $manifest
        $existed = Test-Path -LiteralPath $manifest
        $bytes = $null
        if ($existed) { $bytes = [IO.File]::ReadAllBytes($manifest) }
        $backups += [PSCustomObject]@{ Path=$manifest; Existed=$existed; Bytes=$bytes }
        $lines = @(Get-PackManifestLines $packRoot)
        if ($lines.Count -eq 0) {
            if ($Strict) { throw "No content files found for $version" }
            Write-Host "Skipping $($version): no content files"
            continue
        }
        [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($manifest)) | Out-Null
        [IO.File]::WriteAllLines($manifest, [string[]]$lines, $utf8)
        & $probeExe "valid" $version $packRoot $manifest
        if ($LASTEXITCODE -ne 0) { throw "Valid SHA control failed for $version; no negative result counted." }
        $broken = @($lines | ForEach-Object {
            $prefix = "0"
            if ($_.StartsWith("0")) { $prefix = "f" }
            $prefix + $_.Substring(1)
        })
        [IO.File]::WriteAllLines($manifest, [string[]]$broken, $utf8)
        & $probeExe "mismatch" $version $packRoot $manifest
        if ($LASTEXITCODE -ne 0) { throw "Expected real SHA mismatch was not confirmed for $version." }
        $tested.Add($version) | Out-Null
    }
} finally {
    if (-not $KeepBrokenManifests) {
        foreach ($backup in $backups) {
            if ($backup.Existed) { [IO.File]::WriteAllBytes($backup.Path, [byte[]]$backup.Bytes) }
            elseif (Test-Path -LiteralPath $backup.Path) { Remove-Item -LiteralPath $backup.Path -Force }
        }
    }
    if ($runnerDirectory -and (Test-Path -LiteralPath $runnerDirectory)) { Remove-Item -LiteralPath $runnerDirectory -Recurse -Force }
    Pop-Location
}
Write-Host "Validated failure count: $($tested.Count)"
if ($tested.Count -eq 0) { throw "No content versions were validated." }
Write-Host "All tested content pack versions correctly failed SHA validation with wrong hashes"
exit 0
