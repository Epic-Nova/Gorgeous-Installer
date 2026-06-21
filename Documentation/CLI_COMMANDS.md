# Gorgeous Installer CLI Commands

The Gorgeous Installer provides a robust Command Line Interface (CLI) for automation, continuous integration, and advanced users.

## Usage

```bash
gorgeous-installer [flags] [project_path]
```

By default, the installer attempts to run in GUI mode unless specified otherwise or if executed from an environment without a display.

## Available Flags

### Modes
- `--cli`: Force execution in CLI mode. The installer will run headlessly without showing the graphical UI.
- `--gui`: Force execution in GUI mode.
- `--version-info`: Print build metadata (version, commit hash, date) and exit.

### Core Arguments
- `--project <path>`: The absolute or relative path to the `.uproject` file or the Unreal project directory. Can also be passed as the first positional argument.
- `--type <type>`: Pack type to install (`content` or `code`).
- `--version <version>`: The specific pack version to install or validate (e.g. `1.0.0`).
- `--engine-version <version>`: Engine version used to select the correct pack (e.g. `5.4`).

### Operations
- `--action <action>`: Defines the installation action. Valid options are `install`, `update`, or `reinstall`.
- `--source-dir <path>`: Use a custom local source directory containing plugin files to install instead of the embedded payloads.
- `--install-zip <path>`: Apply an installer update from a downloaded ZIP payload.
- `--recompile-only`: Skip extracting files and exclusively run UnrealBuildTool to compile the plugin binaries for the project.
- `--verify-compatibility`: Opens the GUI mode to resolve binary offset mismatches and initiate a plugin recompile if necessary.

### SHA Validation
- `--validate-sha`: Validates the integrity of pack files against a SHA manifest in CLI mode.
- `--sha-file <path>`: Specifies the exact path to the `manifest.sha256` or `.txt` file to validate against.

### Utility
- `--wait-for-pid <pid>`: Causes the installer to wait until the specified Process ID has terminated before starting. Very useful for updater flows where the caller must exit first.
- `--reopen-project`: Automatically launches the Unreal Editor for the targeted project upon successful compilation or installation.

## Examples

### 1. Headless Installation
Install a specific plugin payload into a project via CLI without showing the UI:
```bash
./gorgeous-installer --cli --action install --engine-version 5.4 --project "C:\UnrealProjects\MyGame\MyGame.uproject"
```

### 2. Validating Pack Integrity
Verify that the downloaded/embedded pack payload matches an external SHA signature file:
```bash
./gorgeous-installer --cli --validate-sha --engine-version 5.4 --sha-file "manifests/release_5.4.sha256"
```

### 3. Recompile Plugin Only
Only re-run UnrealBuildTool to compile the plugin binaries without copying any new payload files, then reopen the project:
```bash
./gorgeous-installer --cli --recompile-only --reopen-project "C:\UnrealProjects\MyGame\MyGame.uproject"
```

### 4. Background Updater Pattern
Wait for an old instance of an application (e.g. PID 4321) to exit, then apply a new update ZIP package:
```bash
./gorgeous-installer --cli --wait-for-pid 4321 --install-zip "update_v1.1.0.zip"
```
