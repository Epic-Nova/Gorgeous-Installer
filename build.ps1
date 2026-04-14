#!/usr/bin/env pwsh
# Build script for Gorgeous Installer (native GUI + embedded assets)

param(
    [switch]$Clean,
    [switch]$Run,
    [switch]$UseUPX,
    [switch]$SkipUPX,
    [switch]$StrictPackSHA,
    [switch]$TestSHAComparison,
    [switch]$Sign,
    [string]$CertPath,
    [string]$CertPassword,
    [string]$TimestampUrl = "http://timestamp.digicert.com"
)

$ErrorActionPreference = "Stop"

# Add Go to PATH
$env:PATH = "C:\Program Files\Go\bin;" + $env:PATH

# Ensure a GCC toolchain is available for CGO (required by Fyne)
$gccInPath = Get-Command gcc -ErrorAction SilentlyContinue
if (-not $gccInPath) {
    $wingetRoot = Join-Path $env:LOCALAPPDATA "Microsoft\WinGet\Packages"
    $winlibsDir = Get-ChildItem $wingetRoot -Directory -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -like "BrechtSanders.WinLibs.POSIX.UCRT_*" } |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1

    if ($winlibsDir) {
        $mingwBin = Join-Path $winlibsDir.FullName "mingw64\bin"
        if (Test-Path (Join-Path $mingwBin "gcc.exe")) {
            $env:PATH = "$mingwBin;" + $env:PATH
        }
    }
}

if (-not (Get-Command gcc -ErrorAction SilentlyContinue)) {
    Write-Host "gcc compiler not found. Install WinLibs or provide gcc in PATH." -ForegroundColor Red
    exit 1
}

$env:CGO_ENABLED = "1"
$env:CC = "gcc"
$env:CXX = "g++"

# Get script directory
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Push-Location $scriptDir

$buildDir = Join-Path $scriptDir "build"
$exePath = Join-Path $buildDir "gorgeous-installer.exe"
$icoPath = Join-Path $buildDir "icon.ico"
$generatedWinresPath = Join-Path $scriptDir "winres.generated.json"
$shaPath = Join-Path $buildDir "gorgeous-installer.exe.sha256"
$sigPath = Join-Path $buildDir "gorgeous-installer.exe.sig"
$metaPath = Join-Path $buildDir "gorgeous-installer.build.json"

if ($UseUPX -and $SkipUPX) {
    Write-Host "-UseUPX and -SkipUPX cannot be used together" -ForegroundColor Red
    Pop-Location
    exit 1
}

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

function Resolve-RelativeToRoot {
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

function New-PackSHAManifest {
    param(
        [string]$PackRoot,
        [string]$ManifestPath
    )

    if (-not (Test-Path $PackRoot)) {
        throw "Pack path not found: $PackRoot"
    }

    $resolvedPackRoot = (Resolve-Path $PackRoot).Path
    $files = Get-ChildItem $resolvedPackRoot -Recurse -File |
        Where-Object { -not (Test-IsSHAControlFile $_.Name) } |
        Sort-Object FullName

    if (-not $files -or $files.Count -eq 0) {
        throw "No pack files found to hash in $resolvedPackRoot"
    }

    $lines = New-Object System.Collections.Generic.List[string]
    foreach ($file in $files) {
        $hash = (Get-FileHash $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        $rel = [System.IO.Path]::GetRelativePath($resolvedPackRoot, $file.FullName)
        $rel = $rel -replace "\\", "/"
        $lines.Add("$hash  $rel") | Out-Null
    }

    $manifestDir = Split-Path -Path $ManifestPath -Parent
    if (-not [string]::IsNullOrWhiteSpace($manifestDir) -and -not (Test-Path $manifestDir)) {
        New-Item -ItemType Directory -Path $manifestDir -Force | Out-Null
    }

    Set-Content -Path $ManifestPath -Value $lines -Encoding Ascii
}

function Resolve-UPXExecutable {
    $upxCmd = Get-Command upx -ErrorAction SilentlyContinue
    if ($upxCmd -and -not [string]::IsNullOrWhiteSpace($upxCmd.Source)) {
        return $upxCmd.Source
    }

    $candidatePaths = New-Object System.Collections.Generic.List[string]
    $wingetLinkPath = Join-Path $env:LOCALAPPDATA "Microsoft\WinGet\Links\upx.exe"
    $candidatePaths.Add($wingetLinkPath) | Out-Null

    $wingetPackagesRoot = Join-Path $env:LOCALAPPDATA "Microsoft\WinGet\Packages"
    if (Test-Path $wingetPackagesRoot) {
        $upxPackage = Get-ChildItem $wingetPackagesRoot -Directory -ErrorAction SilentlyContinue |
            Where-Object { $_.Name -like "UPX.UPX_*" } |
            Sort-Object LastWriteTime -Descending |
            Select-Object -First 1

        if ($upxPackage) {
            $upxBinary = Get-ChildItem $upxPackage.FullName -Recurse -File -Filter "upx.exe" -ErrorAction SilentlyContinue |
                Select-Object -First 1
            if ($upxBinary) {
                $candidatePaths.Add($upxBinary.FullName) | Out-Null
            }
        }
    }

    foreach ($candidate in $candidatePaths) {
        if ([string]::IsNullOrWhiteSpace($candidate)) {
            continue
        }

        if (Test-Path $candidate) {
            return $candidate
        }
    }

    return $null
}

Write-Host "Building Gorgeous Installer..." -ForegroundColor Cyan

if ($Clean) {
    Write-Host "Cleaning previous artifacts..." -ForegroundColor Yellow
    if (Test-Path $buildDir) { Remove-Item $buildDir -Recurse -Force }

    # Remove legacy root artifacts from previous builds.
    if (Test-Path "gorgeous-installer.exe") { Remove-Item "gorgeous-installer.exe" -Force }
    if (Test-Path "gorgeous-installer-debug.exe") { Remove-Item "gorgeous-installer-debug.exe" -Force }
    if (Test-Path "resource.syso") { Remove-Item "resource.syso" -Force }
    if (Test-Path "icon.ico") { Remove-Item "icon.ico" -Force }
    if (Test-Path $generatedWinresPath) { Remove-Item $generatedWinresPath -Force }
}

if (-not (Test-Path $buildDir)) {
    New-Item -ItemType Directory -Path $buildDir | Out-Null
}

if (-not (Test-Path "config.json")) {
    Write-Host "config.json not found in project root" -ForegroundColor Red
    Pop-Location
    exit 1
}

if (-not (Test-Path "packs")) {
    Write-Host "packs folder not found in project root" -ForegroundColor Red
    Pop-Location
    exit 1
}

Write-Host "Validating embedded asset inputs (config.json + packs/**)..." -ForegroundColor Yellow
$packFiles = Get-ChildItem "packs" -Recurse -File -ErrorAction SilentlyContinue
if (-not $packFiles) {
    Write-Host "Warning: packs folder has no files yet; executable will embed an empty packs tree" -ForegroundColor Yellow
}

Write-Host "Generating SHA manifests for configured versions..." -ForegroundColor Yellow
try {
    $configData = Get-Content "config.json" -Raw | ConvertFrom-Json -Depth 20
} catch {
    Write-Host "Failed to parse config.json: $($_.Exception.Message)" -ForegroundColor Red
    Pop-Location
    exit 1
}

if (-not $configData.availableVersions -or $configData.availableVersions.Count -eq 0) {
    Write-Host "config.json has no availableVersions entries" -ForegroundColor Red
    Pop-Location
    exit 1
}

$configUpdated = $false
$skippedSHAVersions = New-Object System.Collections.Generic.List[string]
foreach ($packVersion in $configData.availableVersions) {
    $packPath = [string]$packVersion.path
    if ([string]::IsNullOrWhiteSpace($packPath)) {
        Write-Host "One availableVersions entry has an empty path" -ForegroundColor Red
        Pop-Location
        exit 1
    }

    $shaFile = [string]$packVersion.shaFile
    if ([string]::IsNullOrWhiteSpace($shaFile)) {
        $shaFile = Get-DefaultManifestPath $packPath
        try {
            $packVersion.shaFile = $shaFile
        } catch {
            $packVersion | Add-Member -NotePropertyName shaFile -NotePropertyValue $shaFile -Force
        }
        $configUpdated = $true
    }

    $packRoot = Resolve-RelativeToRoot -Root $scriptDir -PathValue $packPath
    $manifestPath = Resolve-RelativeToRoot -Root $scriptDir -PathValue $shaFile

    try {
        New-PackSHAManifest -PackRoot $packRoot -ManifestPath $manifestPath
        Write-Host "Generated SHA manifest for version $($packVersion.version): $shaFile" -ForegroundColor Green
    } catch {
        $message = "Failed to generate SHA manifest for version $($packVersion.version): $($_.Exception.Message)"
        if ($StrictPackSHA) {
            Write-Host $message -ForegroundColor Red
            Pop-Location
            exit 1
        }

        Write-Host "$message (continuing; use -StrictPackSHA to fail on this)" -ForegroundColor Yellow
        $skippedSHAVersions.Add([string]$packVersion.version) | Out-Null
        continue
    }
}

if ($configUpdated) {
    $configData | ConvertTo-Json -Depth 20 | Set-Content -Path "config.json" -Encoding UTF8
    Write-Host "Updated config.json with generated shaFile entries" -ForegroundColor Green
}

if ($skippedSHAVersions.Count -gt 0) {
    Write-Host "SHA manifests skipped for versions: $([string]::Join(', ', $skippedSHAVersions))" -ForegroundColor Yellow
}

Write-Host "Generating icon file..." -ForegroundColor Yellow
go run .\cmd\iconify\main.go -in icon.png -out $icoPath
if ($LASTEXITCODE -ne 0 -or -not (Test-Path $icoPath)) {
    Write-Host "Failed to generate icon at $icoPath" -ForegroundColor Red
    Pop-Location
    exit 1
}

Write-Host "Compiling executable..." -ForegroundColor Yellow
$gitCommit = "unknown"
$gitCommitFull = ""
try {
    $gitCommitFull = (git rev-parse HEAD 2>$null).Trim()
    if ($LASTEXITCODE -eq 0 -and $gitCommitFull) {
        $gitCommit = $gitCommitFull
    }
} catch {
    $gitCommit = "unknown"
}

$buildTimeUtc = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$ldflags = "-s -w -buildid= -H windowsgui -X gorgeous-installer/internal/buildinfo.CommitSHA=$gitCommit -X gorgeous-installer/internal/buildinfo.BuildTime=$buildTimeUtc"

$winresObject = Get-Content "winres.json" -Raw | ConvertFrom-Json -AsHashtable
$versionInfo = $winresObject["RT_VERSION"]["#1"]["0000"]["info"]["0409"]
$versionInfo["Comments"] = "Commit $gitCommit"
$versionInfo["PrivateBuild"] = $gitCommit
$versionInfo["SpecialBuild"] = $buildTimeUtc
$winresObject | ConvertTo-Json -Depth 20 | Set-Content -Path $generatedWinresPath -Encoding UTF8

go build -trimpath -ldflags $ldflags -o $exePath ./cmd/main
if ($LASTEXITCODE -ne 0 -or -not (Test-Path $exePath)) {
    Write-Host "Build failed" -ForegroundColor Red
    Pop-Location
    exit 1
}

Write-Host "Patching executable resources (icon + version info)..." -ForegroundColor Yellow
go run github.com/tc-hib/go-winres@latest -- patch --in $generatedWinresPath --no-backup $exePath
if ($LASTEXITCODE -ne 0) {
    Write-Host "Resource patch failed" -ForegroundColor Red
    Pop-Location
    exit 1
}

if (Test-Path $generatedWinresPath) {
    Remove-Item $generatedWinresPath -Force
}

if ($UseUPX -or -not $SkipUPX) {
    Write-Host "Applying UPX size optimization..." -ForegroundColor Yellow
    $upxPath = Resolve-UPXExecutable
    if ($upxPath) {
        Write-Host "Using UPX binary: $upxPath" -ForegroundColor Green
        & $upxPath --best --lzma $exePath
        if ($LASTEXITCODE -ne 0) {
            Write-Host "UPX compression failed" -ForegroundColor Red
            Pop-Location
            exit 1
        }
        Write-Host "UPX compression completed" -ForegroundColor Green
    } else {
        if ($UseUPX) {
            Write-Host "UPX was requested but not found in PATH" -ForegroundColor Red
            Pop-Location
            exit 1
        }
        Write-Host "UPX not found in PATH; build continues uncompressed (install UPX or use -UseUPX for strict mode)" -ForegroundColor Yellow
    }
}

if ($Sign) {
    Write-Host "Signing executable..." -ForegroundColor Yellow
    if (-not $CertPath) {
        Write-Host "Signing requested but -CertPath was not provided" -ForegroundColor Red
        Pop-Location
        exit 1
    }

    if (-not (Test-Path $CertPath)) {
        Write-Host "Certificate file not found: $CertPath" -ForegroundColor Red
        Pop-Location
        exit 1
    }

    $signtool = Get-Command signtool.exe -ErrorAction SilentlyContinue
    if (-not $signtool) {
        Write-Host "signtool.exe not found. Install Windows SDK to enable signing." -ForegroundColor Red
        Pop-Location
        exit 1
    }

    $signArgs = @("sign", "/fd", "SHA256", "/f", $CertPath)
    if ($CertPassword) {
        $signArgs += @("/p", $CertPassword)
    }
    $signArgs += @("/tr", $TimestampUrl, "/td", "SHA256", $exePath)

    & $signtool.Source @signArgs
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Code signing failed" -ForegroundColor Red
        Pop-Location
        exit 1
    }
}

Write-Host "Generating checksum and signature artifacts..." -ForegroundColor Yellow
$sha256 = (Get-FileHash $exePath -Algorithm SHA256).Hash.ToLower()
"$sha256 *gorgeous-installer.exe" | Set-Content -Path $shaPath -Encoding Ascii

$signaturePayload = @(
    "algorithm=sha256",
    "file=gorgeous-installer.exe",
    "hash=$sha256",
    "commit=$gitCommit",
    "build_time_utc=$buildTimeUtc"
)
$signaturePayload -join "`n" | Set-Content -Path $sigPath -Encoding Ascii

$goVersion = (go version).Trim()
$meta = [ordered]@{
    file = "gorgeous-installer.exe"
    sha256 = $sha256
    commit = $gitCommit
    buildTimeUtc = $buildTimeUtc
    goVersion = $goVersion
    signed = [bool]$Sign
}
$meta | ConvertTo-Json -Depth 5 | Set-Content -Path $metaPath -Encoding UTF8

$fileSize = (Get-Item $exePath).Length
$fileSizeMB = [math]::Round($fileSize / 1MB, 2)

Write-Host ""
Write-Host "Build successful" -ForegroundColor Green
Write-Host "Binary: $exePath ($fileSizeMB MB)" -ForegroundColor Green
Write-Host "Checksum: $shaPath" -ForegroundColor Green
Write-Host "Signature: $sigPath" -ForegroundColor Green
Write-Host "Metadata: $metaPath" -ForegroundColor Green
Write-Host ""

if ($TestSHAComparison) {
    $shaTestScript = Join-Path $scriptDir "test-sha-comparison.ps1"
    if (-not (Test-Path $shaTestScript)) {
        Write-Host "SHA comparison test script not found: $shaTestScript" -ForegroundColor Red
        Pop-Location
        exit 1
    }

    Write-Host "Running SHA mismatch validation tests..." -ForegroundColor Yellow
    & $shaTestScript -ConfigPath "config.json"
    if ($LASTEXITCODE -ne 0) {
        Write-Host "SHA mismatch validation tests failed" -ForegroundColor Red
        Pop-Location
        exit 1
    }
    Write-Host "SHA mismatch validation tests passed" -ForegroundColor Green
}

if ($Run) {
    Write-Host "Launching Gorgeous Installer..." -ForegroundColor Cyan
    Start-Process $exePath
}

Pop-Location
