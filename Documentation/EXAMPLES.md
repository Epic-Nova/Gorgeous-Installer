# Examples

## Example 1: GUI Code Pack Install

Config snippet:

```json
{
  "pluginName": "GorgeousCore",
  "packType": "code",
  "installPath": "Content",
  "availableVersions": [
    { "version": "5.7", "path": "packs/5.7/content", "checksum": "" },
    { "version": "5.6", "path": "packs/5.6/content", "checksum": "" }
  ]
}
```

User flow:

1. Open installer.
2. Pick Game.uproject.
3. Engine/version detected from EngineAssociation.
4. Install starts.
5. Window expands and live compile log follows output.

Resulting destination for this config when packType=code:

- PLUGIN_PATH/Source

The leading Content segment from installPath is stripped for code installs.

## Example 2: Content Pack Install Path

Config:

```json
{
  "packType": "content",
  "installPath": "Content/Marketplace/MyPack",
  "availableVersions": [
    { "version": "5.4", "path": "packs/5.4/content", "checksum": "" }
  ]
}
```

Result destination:

- PLUGIN_PATH/Content/Marketplace/MyPack

## Example 3: Source Build EngineAssociation

uproject:

```json
{
  "EngineAssociation": "{D:\\UE\\UE5-Main}"
}
```

Behavior:

- association resolves to engine path using direct path/registry lookup
- plugin is then searched in project and engine plugin roots

## Example 4: CLI Automation

```powershell
$projects = @(
  "C:\Games\ProjA\ProjA.uproject",
  "C:\Games\ProjB\ProjB.uproject"
)

foreach ($p in $projects) {
  .\gorgeous-installer.exe -cli -project $p -type code
  if ($LASTEXITCODE -ne 0) {
    Write-Host "Install failed for $p" -ForegroundColor Red
  }
}
```

## Example 5: Compile Fails

If compile fails:

- failure screen appears with message
- screen includes Back to Logs
- log panel keeps compiler output and highlights warnings/errors
- error text advises: Try to rebuild the project from sln project manually

## Example 6: Version Fallback Selection

Available versions: 5.7, 5.6, 5.5
Detected engine: 5.8
Selected pack: 5.7

Rule:

- exact match first
- else newest version not newer than detected engine

## Example 7: Compile Cancellation by Closing

When code compile is running and user closes window:

- close intercept triggers cancellation
- running build process receives cancellation via command context
- app exits cleanly without leaving compile running in background

## Example 8: CLI SHA Validation

Validate a specific pack version with an explicit manifest file:

```powershell
.\gorgeous-installer.exe -validate-sha -version 5.7 -sha-file "C:\Checksums\manifest.sha256"
```

Validate using engine-version-based selection and configured `shaFile`:

```powershell
.\gorgeous-installer.exe -validate-sha -engine-version 5.6
```

## Example 9: Install a Content Example into an Existing Project

Use `packType: "content-example"` for example Blueprints and assets that belong in
an existing project's Content directory. This is distinct from `content`, which
continues to target a plugin's Content directory.

```json
{
  "packName": "GorgeousLoadingScreenExample",
  "packType": "content-example",
  "installPath": "Content",
  "availableVersions": [
    { "version": "5.7", "path": "packs/5.7/example", "shaFile": "packs/5.7.sha256" },
    { "version": "5.6", "path": "packs/5.6/example", "shaFile": "packs/5.6.sha256" }
  ]
}
```

The payload root contains the **contents** of Content, for example
`GorgeousLoadingScreenExample/Maps/Demo.umap`. Do not include a surrounding
Content folder, a .uproject, Config, Source, or Plugins. Asset paths are preserved;
include all referenced assets and install required plugins separately. The installer
does not migrate dependencies, enable plugins, change project settings, or compile.

The user selects their .uproject, and the existing version-selection logic chooses
the packaged payload. Standard numeric EngineAssociation values work even if the
engine is not installed locally. Source-build associations use engine discovery;
if unresolved, the GUI allows manual payload selection. Files go to
`PROJECT/Content/GorgeousLoadingScreenExample/...`, without locating a plugin.
The existing install/update/reinstall comparison applies only to payload files;
unrelated project files are neither deleted nor replaced. Existing files at the
same payload paths can be replaced, so use a unique example namespace.

```powershell
.\Gorgeous-Installer-GorgeousLoadingScreenExample.exe -cli -project "C:\Games\Game\Game.uproject" -version 5.7
```

## Example 10: Full Project Example

Use `packType: "project-example"` for a complete project. Its payload must contain
exactly one .uproject at the root, plus Content and any other required project files.
The GUI offers an empty destination folder and a packaged UE version rather than
asking for an existing .uproject. The CLI uses `-project` for that destination:

```powershell
.\Gorgeous-Installer-FullExample.exe -cli -project "C:\Examples\NewExample" -version 5.7
```

Full projects require an empty or nonexistent destination. This prevents replacing
an existing project's configuration; updating/reinstalling into a populated folder
is intentionally rejected. Files are copied without invoking UnrealBuildTool.

## Build Example Installers in the Publisher UI

1. Open Publisher and select **Examples**.
2. Enter a filesystem-safe example name and choose **Content Example** or **Project Example**.
3. Map each UE version to its source folder. For Content Example, select Content
   itself (its contents are packaged); for Project Example, select the project root.
4. Choose **Build standalone installers** and select an output directory.

This uses the existing offline build pipeline, embedding all versioned payloads and
writing SHA manifests. Content examples do not need SystemManifest.json or a .uplugin.
The builder clones the repository's default branch, so deploy the example support
there before building download installers. Existing build prerequisites apply,
including Bash/Go and the Windows cross-compilation toolchain for .exe output.
The website can distribute the resulting standalone executable; website/API changes
and release binaries are separate from this installer-source change.
