# Examples & Usage Scenarios

## Scenario 1: End User Installing Your Content Pack

User has downloaded `gorgeous-installer.exe` and `config.json` for your content pack.

```powershell
# User opens command prompt and runs
> cd C:\Downloads\GorgeousInstaller
> gorgeous-installer.exe -cli -project "C:\UnrealProjects\MyGame\MyGame.uproject"

# Output:
# Starting installation in CLI mode
# Project: C:\UnrealProjects\MyGame\MyGame.uproject
# Detected UE Version: 5.4
# Engine Path: C:\Program Files\Epic Games\UE_5.4
# Selected Pack Version: 5.4
# Plugin Path: C:\UnrealProjects\MyGame\Plugins\Gorgeous
# Installation completed successfully!
```

## Scenario 2: Supporting Multiple UE Versions

Your config.json supports 4 different UE versions:

```json
{
  "packName": "Gorgeous Premium Assets",
  "packType": "content",
  "installPath": "Content/Marketplace/GorgeousAssets",
  "availableVersions": [
    {"version": "5.4", "path": "packs/5.4"},
    {"version": "5.3", "path": "packs/5.3"},
    {"version": "5.2", "path": "packs/5.2"},
    {"version": "4.27", "path": "packs/4.27"}
  ]
}
```

**Result:**
- UE 5.4 user → Gets 5.4 pack
- UE 5.3 user → Gets 5.3 pack
- UE 5.1 user → Gets 5.2 pack (newest older version)
- UE 4.26 user → Can't install (no compatible pack)

## Scenario 3: Building Installer for Code Pack

You have C++ code for a plugin system:

```powershell
# Step 1: Create configuration
go run cmd/builder/main.go `
  -pack "Gorgeous C++ Systems" `
  -type code `
  -path "D:\Packs\GorgeousCode\Source" `
  -version "5.4"

# Step 2: Build executable
go build -o gorgeous-code-installer.exe cmd/main/main.go

# Step 3: Distribute
# Users receive: gorgeous-code-installer.exe + config.json

# Step 4: User installs
.\gorgeous-code-installer.exe -cli -project "C:\MyProject\MyProject.uproject"

# Output:
# Installing code pack from: D:\Packs\GorgeousCode\Source
# Installation completed successfully
# Note: Plugin requires recompilation in UE Editor
```

## Scenario 4: Source Build Engine Detection

User has a source-built engine registered in Windows:

```
.uproject file contains:
"EngineAssociation": "{D:\\EpicGames\\UE5-Main}"
```

**Installer automatically:**
1. Detects `{}` wrapper = source build
2. Looks up registry: `HKEY_LOCAL_MACHINE\Software\EpicGames\Unreal Engine\SourceBuilds`
3. Finds actual path: `D:\EpicGames\UE5-Main`
4. Uses that path for plugin discovery

```powershell
.\gorgeous-installer.exe -cli -project "C:\GameDev\MyProject\MyProject.uproject"

# Output:
# Detected UE Version: {D:\EpicGames\UE5-Main}
# Engine Path: D:\EpicGames\UE5-Main
# Found plugin at: D:\EpicGames\UE5-Main\Engine\Plugins\Marketplace\Gorgeous
# Installation completed successfully!
```

## Scenario 5: Project-Local vs Engine-Wide Plugin

### Case A: Project has its own Gorgeous plugin
```
MyProject/
├── Plugins/
│   └── Gorgeous/      ← Installer finds and uses this
│       ├── Binaries/
│       ├── Content/
│       └── Source/
└── MyProject.uproject
```

### Case B: Engine-wide Gorgeous plugin
```
C:/Program Files/Epic Games/UE_5.4/
├── Engine/
│   └── Plugins/
│       └── Marketplace/
│           └── Gorgeous/  ← Installer finds if not in project
│               ├── Content/
│               └── Source/
```

**Preference Order:**
1. Project's `Plugins/Gorgeous/`
2. Engine's `Engine/Plugins/Marketplace/Gorgeous/`
3. Engine's `Engine/Plugins/Gorgeous/`

## Scenario 6: Batch Installation Script

Create a PowerShell script for installing to multiple projects:

```powershell
$installer = "C:\Tools\gorgeous-installer.exe"
$projects = @(
    "C:\Games\Project1\Project1.uproject",
    "C:\Games\Project2\Project2.uproject",
    "C:\Games\Project3\Project3.uproject"
)

foreach ($project in $projects) {
    Write-Host "Installing to $project..."
    & $installer -cli -project $project
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✓ Success!" -ForegroundColor Green
    } else {
        Write-Host "✗ Failed with code $LASTEXITCODE" -ForegroundColor Red
    }
}
```

## Scenario 7: Version Matching Examples

Configured versions: `5.4, 5.3, 5.2, 4.27`

| User UE Version | Selected Pack | Reason |
|---|---|---|
| 5.5 | 5.4 | Newest compatible older version |
| 5.4 | 5.4 | Exact match |
| 5.3 | 5.3 | Exact match |
| 5.2.1 | 5.2 | Exact minor version match |
| 5.1 | 5.2 | Can't use 5.3+ (would be incompatible backwards) |
| 5.0 | 4.27 | Newest older major version |
| 4.28 | Can't install | No pack ≤ 4.28 available |

## Scenario 8: Creating Company-Branded Installer

```powershell
# 1. Create your branded config
go run cmd/builder/main.go `
  -pack "Acme Studios Gorgeous Extensions" `
  -type content `
  -path "X:\Releases\AcmeExtensions\Content" `
  -output ".\branched"

# 2. Build branded executable
go build -o AcmeGorgeousInstaller.exe cmd/main/main.go

# 3. Package for distribution
Copy-Item .\branched\config.json .\release\
Copy-Item .\AcmeGorgeousInstaller.exe .\release\

# Users see:
# > .\AcmeGorgeousInstaller.exe
# GUI mode is currently being enhanced. Please use CLI mode:
#   AcmeGorgeousInstaller.exe -cli -project "C:\path\to\project.uproject"
# Available pack versions:
#   - 5.4
```

## Scenario 9: Future - Embedded Assets

Once GUI is complete:

```powershell
# Current workflow:
1. distribute gorgeous-installer.exe
2. distribute config.json separately
3. distribute pack files separately

# Future workflow:  
1. distribute gorgeous-installer.exe ONLY
   - config.json embedded inside
   - pack content streamed or embedded
   - everything self-contained
```

## Scenario 10: CI/CD Pipeline Integration

Automated distribution to team:

```yaml
# .github/workflows/distribute.yml
name: Build & Distribute Installer

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v2
      
      - uses: actions/setup-go@v2
        with:
          go-version: '1.26.2'
      
      - name: Build Installer
        run: |
          go build -o gorgeous-installer.exe cmd/main/main.go
      
      - name: Create Release
        uses: softprops/action-gh-release@v1
        with:
          files: |
            gorgeous-installer.exe
            config.json
```

Team members simply download and run!

---

**Need More Help?**
- See [README.md](README.md) for full documentation
- See [QUICKSTART.md](QUICKSTART.md) for setup
- Check code comments in internal packages for technical details
