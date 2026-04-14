# Gorgeous Installer

Gorgeous Installer is a native Go + Fyne application for installing content and code packs into an Unreal Engine plugin.

It supports both GUI and CLI workflows, resolves the engine from the selected .uproject, chooses a compatible pack version, installs files into the plugin, and (for code packs) compiles the plugin with Unreal build tooling.

## Current Capabilities

- Native desktop GUI (Fyne) with:
  - animated boot intro
  - rounded native Windows window corners
  - draggable custom top bar
  - animated window-open transition
- Engine detection from .uproject EngineAssociation
- Launcher and source build engine path resolution
- Automatic compatible pack version selection
  - version selector is hidden by default
  - selector appears only when engine detection fails
- Content and code pack install modes
- Automatic action detection before install:
  - Install when files are not present yet
  - Update when pack files already exist and differ
  - Reinstall when pack files are already fully installed and unchanged
- SHA validation support:
  - startup validation for configured pack SHA manifests
  - SHA files are excluded from install copy operations
  - animated GUI SHA validation screen (engine-version dropdown + optional manifest file)
  - CLI SHA validation mode with explicit version/engine selection
- Colorized live build log in GUI:
  - errors in red tones
  - warnings in amber tones
  - auto-scroll to newest output
- Update analysis visibility:
  - differing files are listed in the installer log
  - update copies only changed files
- Code-pack compile UX improvements:
  - app window expands with animation for larger log reading space
  - closing while compiling cancels the running compile process
  - compile failures include manual .sln rebuild advice
  - compile failure screen offers Back to Logs
- CLI mode for automation and scripting

## Configuration

The installer reads config.json from embedded assets.

Example schema:

```json
{
  "packName": "Gorgeous Content Pack",
  "pluginName": "GorgeousCore",
  "packType": "code",
  "installPath": "Content",
  "availableVersions": [
    {
      "version": "5.7",
      "path": "packs/5.7/content",
      "shaFile": "packs/5.7/manifest.sha256",
      "checksum": ""
    }
  ]
}
```

Fields:

- packName: human-readable pack name.
- pluginName: target plugin name to locate via .uplugin.
- packType: content or code.
- installPath: path segment appended under plugin Content/ or Source/ roots.
- shaFile: optional checksum manifest for this version (sha256sum format).
- availableVersions: list of installable version payloads.

Important behavior:

- content packs install under PLUGIN_PATH/Content + installPath.
- code packs install under PLUGIN_PATH/Source + installPath.
- for code installs, if installPath starts with Content, that root segment is stripped to avoid Source/Content nesting.
- SHA manifest/control files are not copied into plugin destinations.

## SHA Validation

The installer can validate pack payloads using SHA manifests before installation.

- Startup: configured shaFile manifests are validated against embedded pack files.
- GUI: use Validate SHA to open the validator, select engine version from dropdown, optionally select a manifest, then validate.
- CLI: use `-validate-sha` with `-version` (or `-engine-version`) and optional `-sha-file`.

Manifest line format should follow sha256sum style, for example:

```text
<sha256>  relative/path/to/file.uasset
```

## Engine Detection and Path Resolution

EngineAssociation from the selected .uproject is the source of truth.

Resolution order includes:

- direct path associations (including wrapped {path} forms)
- Windows registry build maps and source build maps
- installed-directory registry keys
- standard install path fallbacks
- Build.version semantic version fallback when association is not semantic

## Plugin Discovery

Plugin lookup order:

1. Project plugin folder
2. Engine Marketplace plugin folder
3. Engine plugin folder

Matching is done by .uplugin file name, using pluginName from config.

## Code Pack Compile Strategy

When packType is code, compilation attempts this order:

1. UnrealBuildTool.dll via dotnet (first choice)
2. UnrealBuildTool executable fallback
3. RunUAT BuildPlugin fallback

The first path is designed to match Unreal/MSBuild-style invocation patterns.

## Build and Run

Build using the workspace script:

```powershell
.\build.ps1
```

Build behavior:

- Generates SHA manifests for every availableVersions path into availableVersions shaFile.
- Missing/empty pack paths are warned and skipped by default.
- Use `-StrictPackSHA` to fail the build if any configured version cannot be hashed.
- Applies UPX compression automatically when UPX is installed.

Build options:

- `-SkipUPX` disables UPX compression.
- `-UseUPX` requires UPX and fails if UPX is not installed.
- `-TestSHAComparison` runs negative SHA tests with intentionally wrong manifests.
- `-Sign -CertPath <pfx> [-CertPassword <pwd>]` signs the exe with signtool.

UPX lookup notes:

- The build script checks PATH first.
- If not found in PATH, it also checks common WinGet install locations.

Run SHA mismatch tests directly:

```powershell
.\test-sha-comparison.ps1
```

Sign an already-built executable:

```powershell
.\sign-build-output.ps1 -CertPath "C:\Path\To\codesign.pfx" -CertPassword "<password>"
```

Artifacts generated in `build/`:

- `gorgeous-installer.exe`
- `gorgeous-installer.exe.sha256`
- `gorgeous-installer.exe.sig`
- `gorgeous-installer.build.json`

Or build directly:

```powershell
go build ./...
```

Run GUI:

```powershell
.\gorgeous-installer.exe
```

Run CLI:

```powershell
.\gorgeous-installer.exe -cli -project "C:\Path\To\Game.uproject"
```

Optional CLI flags:

- -type content|code (defaults to config packType)
- -version X.Y (force a specific pack version)
- -engine-version X.Y (override engine version for pack selection)
- -action install|update|reinstall (force action)
- -validate-sha (run SHA validation mode)
- -sha-file MANIFEST_PATH (override manifest for SHA validation)
- -gui (force GUI mode)
- -version-info (print embedded build metadata)

CLI activation behavior:

- No arguments: GUI mode (desktop/double-click flow).
- CLI arguments present: CLI mode.
- `-cli` always forces CLI mode.

## Code Signing Notes

- You do not need a Microsoft developer account to Authenticode-sign an EXE.
- You do need a code-signing certificate (OV or EV) from a trusted CA.
- EV certificates provide better SmartScreen reputation behavior.

## Inspect EXE Metadata

Use the helper script:

```powershell
.\show-exe-metadata.ps1
```

Or run these directly:

```powershell
(Get-Item .\build\gorgeous-installer.exe).VersionInfo | Format-List *
Get-AuthenticodeSignature .\build\gorgeous-installer.exe | Format-List Status,StatusMessage,SignerCertificate
.\build\gorgeous-installer.exe -version-info
Get-Content .\build\gorgeous-installer.build.json -Raw
```

## Windows Registry Keys Used

Builds and source builds are resolved from common Epic key variants, including:

- HKEY_CURRENT_USER\Software\Epic Games\Unreal Engine\Builds
- HKEY_CURRENT_USER\Software\EpicGames\Unreal Engine\Builds
- HKEY_LOCAL_MACHINE\Software\Epic Games\Unreal Engine\Builds
- HKEY_LOCAL_MACHINE\Software\EpicGames\Unreal Engine\Builds
- HKEY_CURRENT_USER\Software\Epic Games\Unreal Engine\SourceBuilds
- HKEY_CURRENT_USER\Software\EpicGames\Unreal Engine\SourceBuilds
- HKEY_LOCAL_MACHINE\Software\Epic Games\Unreal Engine\SourceBuilds
- HKEY_LOCAL_MACHINE\Software\EpicGames\Unreal Engine\SourceBuilds
- HKEY_LOCAL_MACHINE\Software\WOW6432Node\Epic Games\Unreal Engine\<version>
- HKEY_LOCAL_MACHINE\Software\WOW6432Node\EpicGames\Unreal Engine\<version>

## Documentation

- Quickstart: Documentation/QUICKSTART.md
- Examples: Documentation/EXAMPLES.md
- Pack folder notes: packs/README.md

## Repository Layout

```text
cmd/                Entry points and helper tools
internal/config/    Config loading and defaults
internal/unreal/    Unreal project and engine detection
internal/registry/  Engine association/path resolution
internal/installer/ Install and compile pipeline
internal/ui/        Native GUI and window behavior
packs/              Embedded pack payload roots
Documentation/      User guides and usage scenarios
```
