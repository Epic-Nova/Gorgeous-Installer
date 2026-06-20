#!/usr/bin/env pwsh
# Negative SHA validation test: intentionally writes wrong hashes for content packs and ensures validation fails.

param(
    [string]$ConfigPath = ".\\config.json",
    [switch]$Strict,
    [switch]$KeepBrokenManifests
)

$ErrorActionPreference = "Stop"

function Test-IsSHAControlFile {
    param([string]$FileName)

    if ([string]::IsNullOrWhiteSpace($FileName)) {
        return $false
    }

    $name = $FileName.ToLowerInvariant()
    return $name.EndsWith(".sha") -or
        $name.EndsWith(".sha256") -or
        $name.EndsWith(".sha512") -or
        $name -eq "sha256.txt" -or
        $name -eq "checksums.txt" -or
        $name -eq "manifest.sha256"
}

function Resolve-AbsolutePath {
    param(
        [string]$Root,
        [string]$PathValue
    )

    if ([System.IO.Path]::IsPathRooted($PathValue)) {
        return [System.IO.Path]::GetFullPath($PathValue)
    }

    return [System.IO.Path]::GetFullPath((Join-Path $Root $PathValue))
}

function Get-DefaultManifestPath {
    param([string]$PackPath)

    $normalized = ($PackPath -replace "\\", "/").Trim()
    $baseDir = Split-Path -Path $normalized -Parent
    if ([string]::IsNullOrWhiteSpace($baseDir)) {
        return "$normalized.sha256"
    }

    return "$baseDir/manifest.sha256"
}

function Get-PackManifestLines {
    param([string]$PackRoot)

    $resolvedPackRoot = (Resolve-Path $PackRoot).Path
    $files = Get-ChildItem $resolvedPackRoot -Recurse -File |
        Where-Object { -not (Test-IsSHAControlFile $_.Name) } |
        Sort-Object FullName

    if (-not $files -or $files.Count -eq 0) {
        return @()
    }

    $lines = New-Object System.Collections.Generic.List[string]
    foreach ($file in $files) {
        $hash = (Get-FileHash $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        $rel = [System.IO.Path]::GetRelativePath($resolvedPackRoot, $file.FullName)
        $rel = $rel -replace "\\", "/"
        $lines.Add("$hash  $rel") | Out-Null
    }

    return $lines
}

function Convert-ToBrokenManifestLines {
    param([string[]]$ValidLines)

    $broken = New-Object System.Collections.Generic.List[string]
    foreach ($line in $ValidLines) {
        if ([string]::IsNullOrWhiteSpace($line)) {
            continue
        }

        $parts = $line -split "\s+", 2
        if ($parts.Count -lt 2) {
            continue
        }

        $hash = $parts[0].Trim().ToLowerInvariant()
        $filePath = $parts[1].Trim()
        if ($hash.Length -ne 64) {
            continue
        }

        $replacementPrefix = "0"
        if ($hash.StartsWith("0")) {
            $replacementPrefix = "f"
        }

        $wrongHash = $replacementPrefix + $hash.Substring(1)
        $broken.Add("$wrongHash  $filePath") | Out-Null
    }

    return $broken
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Push-Location $scriptDir

$manifestBackups = @()
$testedVersions = New-Object System.Collections.Generic.List[string]
$skippedVersions = New-Object System.Collections.Generic.List[string]
$failedVersions = New-Object System.Collections.Generic.List[string]

try {
    if (-not (Test-Path $ConfigPath)) {
        Write-Host "Config file not found: $ConfigPath" -ForegroundColor Red
        exit 1
    }

    $config = Get-Content $ConfigPath -Raw | ConvertFrom-Json -Depth 20
    if (-not $config.availableVersions -or $config.availableVersions.Count -eq 0) {
        Write-Host "No availableVersions were found in $ConfigPath" -ForegroundColor Red
        exit 1
    }

    foreach ($versionEntry in $config.availableVersions) {
        $version = [string]$versionEntry.version
        $packPath = [string]$versionEntry.path
        if ([string]::IsNullOrWhiteSpace($version) -or [string]::IsNullOrWhiteSpace($packPath)) {
            $msg = "Skipping one availableVersions entry with empty version/path"
            if ($Strict) {
                Write-Host $msg -ForegroundColor Red
                exit 1
            }

            Write-Host $msg -ForegroundColor Yellow
            continue
        }

        $normalizedPackPath = ($packPath -replace "\\", "/").ToLowerInvariant()
        if (-not $normalizedPackPath.Contains("/content")) {
            Write-Host "Skipping ${version}: path is not a content pack path ($packPath)" -ForegroundColor Yellow
            $skippedVersions.Add($version) | Out-Null
            continue
        }

        $packRoot = Resolve-AbsolutePath -Root $scriptDir -PathValue $packPath
        if (-not (Test-Path $packRoot)) {
            $msg = "Skipping ${version}: pack path not found ($packPath)"
            if ($Strict) {
                Write-Host $msg -ForegroundColor Red
                exit 1
            }

            Write-Host $msg -ForegroundColor Yellow
            $skippedVersions.Add($version) | Out-Null
            continue
        }

        $shaFile = [string]$versionEntry.shaFile
        if ([string]::IsNullOrWhiteSpace($shaFile)) {
            $shaFile = Get-DefaultManifestPath $packPath
        }

        $manifestPath = Resolve-AbsolutePath -Root $scriptDir -PathValue $shaFile
        $manifestExists = Test-Path $manifestPath
        $manifestContent = $null
        if ($manifestExists) {
            $manifestContent = Get-Content $manifestPath -Raw
        }

        $manifestBackups += [PSCustomObject]@{
            Version = $version
            ManifestPath = $manifestPath
            Exists = $manifestExists
            Content = $manifestContent
        }

        $manifestLines = Get-PackManifestLines -PackRoot $packRoot
        if (-not $manifestLines -or $manifestLines.Count -eq 0) {
            $msg = "Skipping ${version}: no content files found for hashing"
            if ($Strict) {
                Write-Host $msg -ForegroundColor Red
                exit 1
            }

            Write-Host $msg -ForegroundColor Yellow
            $skippedVersions.Add($version) | Out-Null
            continue
        }

        $brokenLines = Convert-ToBrokenManifestLines -ValidLines $manifestLines
        if (-not $brokenLines -or $brokenLines.Count -eq 0) {
            $msg = "Skipping ${version}: could not generate broken SHA lines"
            if ($Strict) {
                Write-Host $msg -ForegroundColor Red
                exit 1
            }

            Write-Host $msg -ForegroundColor Yellow
            $skippedVersions.Add($version) | Out-Null
            continue
        }

        $manifestDir = Split-Path -Path $manifestPath -Parent
        if (-not [string]::IsNullOrWhiteSpace($manifestDir) -and -not (Test-Path $manifestDir)) {
            New-Item -ItemType Directory -Path $manifestDir -Force | Out-Null
        }

        Set-Content -Path $manifestPath -Value $brokenLines -Encoding Ascii
        Write-Host "Injected wrong SHA manifest for version ${version}: $shaFile" -ForegroundColor Yellow

        & go run ./cmd/main -validate-sha -version $version
        $exitCode = $LASTEXITCODE

        if ($exitCode -eq 0) {
            Write-Host "Unexpected success for version $version; SHA mismatch test should fail" -ForegroundColor Red
            $failedVersions.Add($version) | Out-Null
            continue
        }

        Write-Host "Expected SHA validation failure observed for version $version" -ForegroundColor Green
        $testedVersions.Add($version) | Out-Null
    }
} finally {
    if (-not $KeepBrokenManifests) {
        foreach ($backup in $manifestBackups) {
            if ($backup.Exists) {
                [System.IO.File]::WriteAllText($backup.ManifestPath, [string]$backup.Content, [System.Text.Encoding]::ASCII)
            } else {
                if (Test-Path $backup.ManifestPath) {
                    Remove-Item $backup.ManifestPath -Force
                }
            }
        }

        if ($manifestBackups.Count -gt 0) {
            Write-Host "Original SHA manifest files were restored" -ForegroundColor Green
        }
    } else {
        Write-Host "Keeping broken manifests because -KeepBrokenManifests was set" -ForegroundColor Yellow
    }

    Pop-Location
}

Write-Host ""
Write-Host "SHA mismatch test summary" -ForegroundColor Cyan
Write-Host "Validated failure count: $($testedVersions.Count)" -ForegroundColor Cyan
if ($skippedVersions.Count -gt 0) {
    Write-Host "Skipped versions: $([string]::Join(', ', $skippedVersions))" -ForegroundColor Yellow
}
if ($failedVersions.Count -gt 0) {
    Write-Host "Unexpected pass versions: $([string]::Join(', ', $failedVersions))" -ForegroundColor Red
    exit 1
}

if ($testedVersions.Count -eq 0) {
    Write-Host "No versions were validated. Add at least one content pack path with files." -ForegroundColor Red
    exit 1
}

Write-Host "All tested content pack versions correctly failed SHA validation with wrong hashes" -ForegroundColor Green
exit 0
