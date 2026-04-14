# Quickstart

This guide gets you from config to a working install quickly.

## 1) Prepare Config

Edit config.json:

```json
{
  "packName": "Gorgeous Content Pack",
  "pluginName": "GorgeousCore",
  "packType": "code",
  "installPath": "Content",
  "availableVersions": [
    { "version": "5.7", "path": "packs/5.7/content", "shaFile": "packs/5.7/manifest.sha256", "checksum": "" }
  ]
}
```

Notes:

- packType controls install + compile behavior.
- installPath is appended under plugin Content/ or Source/ roots.
- shaFile enables startup/GUI/CLI SHA validation for each version.
- build.ps1 auto-generates each configured shaFile from the files in that version path.

## 2) Place Pack Payload

Put pack files under the configured path roots, for example:

- packs/5.7/content/...
- packs/5.6/content/...

These files are embedded at build time.

## 3) Build

```powershell
.\build.ps1
```

Or:

```powershell
go build ./...
```

## 4) Run GUI

```powershell
.\gorgeous-installer.exe
```

Typical GUI flow:

1. Select a .uproject file.
2. Engine/version is auto-detected from EngineAssociation.
3. Compatible pack version is auto-selected.
4. Install starts and logs stream live.

Before install starts, the app compares pack files against the target plugin path:

- Install button stays Install for fresh targets.
- Button switches to Update when existing files differ.
- For already-fully-installed packs with unchanged files, button switches to Reinstall.
- Differing files are listed in the log, and Update copies only changed files.

SHA validation in GUI:

- Click Validate SHA.
- Select an engine version from the dropdown (from availableVersions).
- Optionally browse to a custom SHA file.
- Click Validate.

Code pack specifics:

- window expands with animation for log readability
- errors/warnings are colorized
- log auto-scrolls to newest lines
- closing while compiling cancels compile

## 5) Run CLI (Optional)

```powershell
.\gorgeous-installer.exe -cli -project "C:\Path\To\Game.uproject"
```

Optional:

```powershell
.\gorgeous-installer.exe -cli -project "C:\Path\To\Game.uproject" -type code
```

SHA validation mode:

```powershell
.\gorgeous-installer.exe -validate-sha -version 5.7 -sha-file "C:\Path\To\manifest.sha256"
```

or version-by-engine selection:

```powershell
.\gorgeous-installer.exe -validate-sha -engine-version 5.7
```

## Install Destinations

- content: PLUGIN_PATH/Content + installPath
- code: PLUGIN_PATH/Source + installPath

If installPath starts with Content during code install, that root segment is stripped so code does not end up in Source/Content unless explicitly nested beyond root.

## Code Compile Order

For code packs, compile attempts:

1. dotnet + UnrealBuildTool.dll (first)
2. UnrealBuildTool executable
3. RunUAT BuildPlugin fallback

If compile fails, the app shows failure details and advises manual rebuild from the solution project.

## Troubleshooting

Engine not found:

- verify .uproject EngineAssociation
- verify registry keys for launcher/source builds

Plugin not found:

- ensure pluginName in config matches your .uplugin name
- ensure plugin exists in project or engine plugin folders

Compile failure:

- use Back to Logs in the failure screen
- inspect colored log output
- try manual rebuild from .sln project
