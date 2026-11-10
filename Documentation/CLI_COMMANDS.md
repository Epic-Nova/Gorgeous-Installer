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
- `--project <path>`: The absolute or relative path to the `.uproject` file or Unreal project directory. A `project-example` install instead requires an empty destination directory. Can also be passed as the first positional argument.
- `--type <type>`: `content`, `code`, `hybrid`, `content-example`, `project-example`, or `game-feature-example`.
- `--version <version>`: The configured pack version to install or validate (for example `5.7` for a UE 5.7 payload mapping).
- `--engine-version <version>`: Engine version used to select the correct pack (e.g. `5.4`).

### Operations
- `--action <action>`: Defines the installation action. Valid options are `install`, `update`, or `reinstall`.
- `--source-dir <path>`: Use a custom local source directory containing plugin files to install instead of the embedded payloads.
- `--install-zip <path>`: Apply a local plugin-update ZIP to the specified project and explicitly selected `--target-plugin`. This is separate from installer binary self-updates.
- `--recompile-only`: Skip extracting files and exclusively run UnrealBuildTool to compile the plugin binaries for the project.
- `--verify-compatibility`: Opens the GUI mode to resolve binary offset mismatches and initiate a plugin recompile if necessary.
- `--regenerate-project`: Opens the project-file regeneration/recompile workflow.
- `--target-plugin <name>`: Explicitly binds a ZIP update to the intended plugin.

### Network access
- `--allow-network`: Explicitly allow HTTPS API requests for this invocation.
- `--offline`: Disable all API access, overriding network consent.

Network access is off by default. Local installs, SHA validation and compilation do not require API permission.

### Windows self-installation
- `--install-self`: Install for the current user, including a Start Menu shortcut and Installed apps registration.
- `--uninstall-self`: Remove the recorded installation and its owned shell integration.
- `--self-install-dir <directory>`: Override the configured directory for these self-install/uninstall operations.

These operations run before loading embedded pack configuration or initializing network access. They do not enable sign-in autostart or silently replace Windows' protected default-app choice. See [Windows workflows](Windows.md).

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

### 4. Plugin Update After Editor Exit
Wait for Unreal Editor (e.g. PID 4321) to exit, then apply a plugin update to its selected project/plugin:
```bash
./gorgeous-installer --cli --wait-for-pid 4321 --project "/projects/MyGame/MyGame.uproject" --target-plugin GorgeousCore --install-zip "update_v1.1.0.zip"
```
