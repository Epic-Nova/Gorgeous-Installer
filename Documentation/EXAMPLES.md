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
