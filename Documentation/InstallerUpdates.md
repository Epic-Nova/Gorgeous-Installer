# Managing Installer Updates

The Gorgeous Installer supports self-updating. Because the installer functions both as a pre-compiled native GUI client and as a local raw-source plugin embedded in your Unreal projects, there are two distinct update tracks.

## Update Tracks

1. **Installer Source Update** (`GorgeousInstaller-Source`): Updates the raw Go source files. This is intended for local plugin versions of the installer embedded inside Unreal Engine projects. 
2. **Installer Binary Update** (`GorgeousInstaller-Bin`): Updates the compiled native executable files (`gorgeous-installer` or `gorgeous-installer.exe`). This is intended for standalone native installations of the GUI app.

## How to Bump the Installer Version

The host build script generates the installer's version metadata: `build.ps1` on Windows and `build.sh` on Linux. The publisher's rebuild action updates the version inputs for the host before invoking that script.

| Host | Version input |
| --- | --- |
| Linux | `VERSION="1.0.0"` in `build.sh` |
| Windows | Product version in `winres.json`; fixed Windows resource versions must stay numeric, while the display product version can identify a prerelease |

Update the input or use the publisher's version/rebuild action, then run `./build.sh` on Linux or `.\build.ps1` on Windows. The script compiles into `build/` and generates `internal/buildinfo/buildinfo.go`. Direct Go builds reuse the last generated metadata.

## How to Publish Updates

1. Open the Gorgeous Installer.
2. Ensure you have **Developer Mode** enabled in the Settings to reveal the Publisher Panel.
3. Navigate to the Publisher Panel.
4. Select the **Installer Source Update** or **Installer Binary Update** radio button, depending on which track you want to update.
5. Provide Release Notes in the provided text area.
6. Click **Sign & Publish**.

> [!IMPORTANT]
> When publishing a **Binary Update**, the publisher ZIPs the contents of the local `build/` directory. Run the host build script first so that the intended executables and metadata are included. Windows publishing does not require a Linux binary, Bash or an external ZIP utility.

## How Clients Receive Updates

- When network access has been explicitly enabled, the installer checks the API (`GET /api/v1/installer/update-check`) for its update track. Offline mode overrides network consent.
- If an update is available, a toast notification appears.
- When clicked, the user can review the Release Notes and start the update.
- Both tracks require HTTPS, a matching SHA-256 digest and a release signature verified against the configured trust key before applying downloaded contents.
- **Binary Updates** locate the host executable in a private staging directory. Windows uses a hidden PID-aware helper, preserves the old executable for rollback and records failures before relaunch. Linux retains its shell handoff in the update's private directory.
- **Source Updates** locate the `gorgeous-installer` module from the executable/working-directory ancestry and update that source tree, rather than assuming the launcher's current directory is the plugin directory. They do not rebuild the currently running executable.

See [Windows workflows](Windows.md) for installation, uninstall and native regression checks. Trust-key provisioning and publishing to the production service are separate operator actions.

> [!NOTE]
> If **Developer Mode** is currently enabled and the installer is running from a local codebase (Source mode), the installer will **never** check for Source Updates. This ensures that any local modifications you are making to the installer's source code are not accidentally overwritten.
