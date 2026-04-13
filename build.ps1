#!/usr/bin/env pwsh
# Build script for Gorgeous Installer with Windows resources

param(
    [switch]$Clean,
    [switch]$Run
)

$ErrorActionPreference = "Stop"

# Add Go to PATH
$env:PATH = "C:\Program Files\Go\bin;" + $env:PATH

# Get script directory
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Push-Location $scriptDir

Write-Host "🔨 Building Gorgeous Installer..." -ForegroundColor Cyan

# Clean if requested
if ($Clean) {
    Write-Host "🧹 Cleaning old builds..." -ForegroundColor Yellow
    if (Test-Path "resource.syso") { Remove-Item resource.syso -Force }
    if (Test-Path "gorgeous-installer.exe") { Remove-Item gorgeous-installer.exe -Force }
}

# Generate Windows resources with icon and metadata
Write-Host "📦 Generating Windows resources..." -ForegroundColor Yellow
try {
    go run github.com/tc-hib/go-winres@latest make --in winres.json 2>&1 | ForEach-Object {
        if ($_ -match "error|failed") {
            Write-Host "⚠️  gowinres: $_" -ForegroundColor Gray
        }
    }
    if (Test-Path "resource.syso") {
        Write-Host "✓ Resource file created" -ForegroundColor Green
    }
} catch {
    Write-Host "⚠️  Could not generate resources (continuing without icon embedding)" -ForegroundColor Yellow
}

# Build the executable
Write-Host "⚙️  Compiling executable..." -ForegroundColor Yellow
go build -ldflags "-s -w -H windowsgui" -o gorgeous-installer.exe cmd/main/main.go

if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Build failed!" -ForegroundColor Red
    Pop-Location
    exit 1
}

$fileSize = (Get-Item gorgeous-installer.exe).Length
$fileSizeMB = [math]::Round($fileSize / 1MB, 2)

Write-Host ""
Write-Host "✓ Build successful!" -ForegroundColor Green
Write-Host "  Binary: gorgeous-installer.exe ($fileSizeMB MB)" -ForegroundColor Green
Write-Host "  Location: $(Get-Item gorgeous-installer.exe | Select-Object -ExpandProperty FullName)" -ForegroundColor Green
Write-Host ""

# Run if requested
if ($Run) {
    Write-Host "🚀 Launching Gorgeous Installer..." -ForegroundColor Cyan
    Start-Process .\gorgeous-installer.exe
}

Pop-Location
