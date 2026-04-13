# Gorgeous Unreal Engine Plugin Installer

A Go-based installer tool for distributing Unreal Engine plugin content and code packs. Supports both GUI and CLI modes, automatic engine version detection, and intelligent version matching.

## Features

- **Dual-Mode Operation**: Web-based GUI for interactive installation, CLI for automation
- **Smart Version Matching**: Automatically selects compatible pack versions, preferring older versions when exact match unavailable
- **Unreal Engine Detection**: Automatically detects UE version from .uproject files
- **Plugin Location**: Finds Gorgeous plugin in project or engine directories
- **Source Build Support**: Handles source-built engines identified by `{SourcePath}` notation
- **Registry Support**: Uses Windows registry to locate engine installations and source builds
- **Content & Code Packs**: Supports both content (goes to Content/) and code (goes to Source/, auto-recompiles) packs
- **Cross-Platform**: Works on Windows, macOS, and Linux
- **Beautiful Web Interface**: Animated, rounded-corner UI styled to match Gorgeous Core branding

## Project Structure

```
gorgeous-installer/
├── cmd/
│   ├── main/
│   │   └── main.go           # GUI and CLI application entry point
│   └── builder/
│       └── main.go           # Tool to build installer packages
├── internal/
│   ├── config/
│   │   └── config.go         # Configuration management
│   ├── unreal/
│   │   └── unreal.go         # UE-specific operations
│   ├── registry/
│   │   └── registry.go       # Engine registry lookups
│   ├── installer/
│   │   └── installer.go      # Installation logic
│   └── ui/
│       └── web.go            # Web-based GUI (HTML/CSS/JavaScript)
├── deployments/              # Where packaged content/code goes
├── config.json              # Installer configuration (embedded in exe)
└── README.md
```

## Prerequisites

- **Go 1.26.2** or later
- **Windows registry access** (for engine location detection)
- **Unreal Engine 4.27+** (target engine)
- **A modern web browser** (for the GUI - Chrome, Firefox, Edge, Safari all supported)

## Building & Installation

### 1. Setup Development Environment

```powershell
# Install Go dependencies
cd "h:\SimsalabimStudio\Applications\GorgeousInstaller"
go mod tidy
go mod download
```

### 2. Configure Your Pack

Create or update `config.json` with your pack information:

```json
{
  "packName": "My Gorgeous Pack",
  "packType": "content",
  "installPath": "Content",
  "availableVersions": [
    {
      "version": "5.4",
      "path": "packs/5.4/content",
      "checksum": ""
    },
    {
      "version": "5.3",
      "path": "packs/5.3/content",
      "checksum": ""
    }
  ]
}
```

**Pack Types:**
- `content`: Files installed to `Plugins/Gorgeous/Content/`
- `code`: Files installed to `Plugins/Gorgeous/Source/`, triggers plugin recompilation

### 3. Build Configuration Tool

Build the builder tool to create pre-configured installers:

```powershell
$env:PATH = "C:\Program Files\Go\bin;" + $env:PATH

# Build configuration for a content pack
go run cmd/builder/main.go -pack "MyContentPack" -type content -path "C:\MyPacks\Content" -output . -version "5.4"

# For code packs
go run cmd/builder/main.go -pack "MyCodePack" -type code -path "C:\MyPacks\Source" -output . -version "5.4"
```

### 4. Build the Installer Executable

#### GUI Mode (Default)
```powershell
$env:PATH = "C:\Program Files\Go\bin;" + $env:PATH
go build -o gorgeous-installer.exe cmd/main/main.go
```

#### With Embedded Assets
To bundle config.json and pack content:
```powershell
# Place config.json and pack content in same directory as exe
go build -o gorgeous-installer.exe cmd/main/main.go
```

## Usage

### GUI Mode (Web Interface)

Simply run the executable:

```powershell
.\gorgeous-installer.exe
```

This will:
1. Start a local web server on `http://localhost:8765`
2. Automatically open your default browser
3. Display the beautifully styled Gorgeous Core interface

**Workflow:**
1. Copy and paste your `.uproject` file path (or use the browse button)
2. Installer auto-detects engine version from the project
3. Select desired pack version from dropdown
4. Click "⚙️ Install Pack" to begin installation
5. Monitor status message for success/error feedback

**Design Features:**
- Animated UI with smooth transitions
- Rounded corners throughout (20px container, 12px cards)
- Gorgeous Core teal/pink color scheme
- Responsive layout that works on any screen size
- Real-time status updates during installation

### CLI Mode

```powershell
.\gorgeous-installer.exe -cli -project "C:\MyProject\MyProject.uproject" -type content
```

**CLI Arguments:**
- `-cli`: Enable CLI mode (optional, defaults to GUI)
- `-project`: Path to .uproject file or project directory (required in CLI)
- `-type`: Pack type (optional, uses config.packType if not specified)

**CLI Output Example:**
```
Starting installation in CLI mode
Project: C:\MyProject\MyProject.uproject
Detected UE Version: 5.4
Engine Path: C:\Program Files\Epic Games\UE_5.4
Selected Pack Version: 5.4
Plugin Path: C:\MyProject\Plugins\Gorgeous
Installation completed successfully!
```

## Engine Version Detection

### Standard Installations
Automatically locates UE from registry using version string:
- `5.4` → Finds: `C:\Program Files\Epic Games\UE_5.4`
- `5.3` → Finds: `C:\Program Files\Epic Games\UE_5.3`

### Source Builds
Versions in curly braces `{path}` are treated as source builds:
- `{C:\UnrealEngine}` → Uses path from registry or direct path
- `{D:\EpicGames\UE5}` → Looks up in registry: `HKEY_LOCAL_MACHINE\Software\EpicGames\Unreal Engine\SourceBuilds`

### Lookup Locations

**Windows:**
```
HKEY_LOCAL_MACHINE\Software\Epic Games\Unreal Engine\Builds
HKEY_CURRENT_USER\Software\Epic Games\Unreal Engine\Builds
```

**macOS/Linux:**
```
$HOME/UnrealEngine/<version>
$HOME/UE/<version>
/opt/UnrealEngine/<version>
```

## Plugin Discovery

The installer searches for the Gorgeous plugin in this order:

1. **Project Plugins**: `{ProjectPath}/Plugins/Gorgeous/`
2. **Engine Marketplace**: `{EnginePath}/Engine/Plugins/Marketplace/Gorgeous/`
3. **Engine Plugins**: `{EnginePath}/Engine/Plugins/Gorgeous/`

## Installation Details

### Content Pack Installation
- Copies entire pack to: `Plugins/Gorgeous/Content/{installPath}/`
- Does NOT require recompilation
- Files are immediately available in Unreal Editor

### Code Pack Installation
- Copies code to: `Plugins/Gorgeous/Source/{installPath}/`
- Attempts automatic recompilation using UnrealBuildTool
- If recompilation fails, files are still installed (manual recompile option follows)

## Packaging for Distribution

### Step 1: Prepare Pack Contents
```
MyPackContent/
├── Meshes/
├── Materials/
├── Textures/
└── ...
```

### Step 2: Create Configuration
```powershell
go run cmd/builder/main.go `
  -pack "My Pack" `
  -type content `
  -path "C:\path\to\MyPackContent" `
  -version "5.4"
```

### Step 3: Build Executable
```powershell
go build -o MyPackInstaller.exe cmd/main/main.go
```

### Step 4: Distribute
- Package the .exe with config.json
- Optionally include pack files if not yet embedded
- Provide to users for installation

## Version Matching Algorithm

When user selects a project with UE version X.Y:

1. **Exact Match**: If pack version X.Y exists → use it
2. **No Match**: Iterate through available versions:
   - Find versions older than X.Y
   - Select the newest (most recent) older version
   - Example: UE 5.5 available, packs exist for 5.4 and 5.3 → install 5.4

## Error Handling

The installer provides clear error messages for:
- Missing .uproject files
- Unreachable engine installations
- Missing Gorgeous plugin
- File permission issues
- Invalid configuration

Example error handling:
```powershell
# Will show user-friendly error dialog
# and log detailed information to stderr in CLI mode
```

## Development

### Adding New Features

**Registry Support for New Platforms:**
1. Extend `internal/registry/registry.go`
2. Implement platform-specific path lookup
3. Add to appropriate OS detection

**Custom UI Styling:**
1. Modify `internal/ui/gui.go`
2. Implement custom Fyne canvas objects for rounded corners
3. Add theme support

### Building for Other Platforms

```powershell
# macOS
$env:GOOS = "darwin"; $env:GOARCH = "amd64"
go build -o gorgeous-installer-macos cmd/main/main.go

# Linux
$env:GOOS = "linux"; $env:GOARCH = "amd64"
go build -o gorgeous-installer-linux cmd/main/main.go
```

## Troubleshooting

### "Engine version not found in registry"
- Verify UE installation is in standard location
- Check registry: `regedit` → Navigate to engine registry paths
- For source builds, verify `{path}` is correct in .uproject

### "Gorgeous plugin not found"
- Ensure plugin is installed in project or engine
- Check plugin path: Should contain `.uplugin` file
- Verify folder naming (case-sensitive on some systems)

### Plugin recompilation fails
- Ensure Visual Studio/C++ tools are installed
- Verify UnrealBuildTool accessibility
- Check plugin Source folder for syntax errors

## License

This tool is part of the Gorgeous Plugin ecosystem.

## Support

For issues and feature requests, contact the development team or check codebase documentation.
