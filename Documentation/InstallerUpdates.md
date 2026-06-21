# Managing Installer Updates

The Gorgeous Installer supports self-updating. Because the installer functions both as a pre-compiled native GUI client and as a local raw-source plugin embedded in your Unreal projects, there are two distinct update tracks.

## Update Tracks

1. **Installer Source Update** (`GorgeousInstaller-Source`): Updates the raw Go source files. This is intended for local plugin versions of the installer embedded inside Unreal Engine projects. 
2. **Installer Binary Update** (`GorgeousInstaller-Bin`): Updates the compiled native executable files (`gorgeous-installer` or `gorgeous-installer.exe`). This is intended for standalone native installations of the GUI app.

## How to Bump the Installer Version

The installer version is managed by the `build.sh` script.

1. Open `build.sh`.
2. Locate the line near the top:
   ```bash
   VERSION="1.0.0"
   ```
3. Update this version number to your desired new version (e.g., `"1.0.1"`).
4. Run `./build.sh`. This process will compile the new binaries into the `build/` folder and generate the internal `buildinfo.go` file used to declare the running version.

## How to Publish Updates

1. Open the Gorgeous Installer.
2. Ensure you have **Developer Mode** enabled in the Settings to reveal the Publisher Panel.
3. Navigate to the Publisher Panel.
4. Select the **Installer Source Update** or **Installer Binary Update** radio button, depending on which track you want to update.
5. Provide Release Notes in the provided text area.
6. Click **Sign & Publish**.

> [!IMPORTANT]
> When publishing a **Binary Update**, the publisher will specifically ZIP the contents of the local `build/` directory. Ensure you have successfully run `./build.sh` prior to publishing so that your newest compiled executables are included!

## How Clients Receive Updates

- When the installer launches, it checks the API (`GET /api/v1/installer/update-check`) to see if its specific track has a newer version than its current `buildinfo.Version`.
- If an update is available, a toast notification appears.
- When clicked, the user can review the Release Notes and start the update.
- **Binary Updates** download the zip, locate the correct executable for the host OS, and silently swap the running executable via a background shell script before restarting.
- **Source Updates** download the zip and directly overwrite the local `.go` source files in the plugin directory.

> [!NOTE]
> If **Developer Mode** is currently enabled and the installer is running from a local codebase (Source mode), the installer will **never** check for Source Updates. This ensures that any local modifications you are making to the installer's source code are not accidentally overwritten.
