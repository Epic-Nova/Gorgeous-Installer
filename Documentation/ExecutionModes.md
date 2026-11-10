# Gorgeous Installer: Execution Modes

Because the Gorgeous Installer is designed to handle so many different entry points seamlessly, it operates through several distinct **Execution Modes**. Here is a comprehensive guide to every mode the installer supports, how it is triggered, and its intended use case.

## 1. Auto-Build Mode (Focus Mode)
**How it triggers:** By opening a `.uproject` through Gorgeous Installer's registered file association or project workflow. On Windows the app registers an Open with choice; making it the default may require explicit selection in Windows Default apps.

**Behavior:**
Bypasses the main installer dashboard completely. It evaluates the Unreal project, detects if the binaries are missing or outdated, and instantly opens a dark, centered "Focus Mode" compilation modal. Once compilation finishes successfully, it automatically launches the Unreal Engine Editor and closes itself.

**Use Case:**
Day-to-day frictionless workflow for the development team. Developers never have to manually run `GenerateProjectFiles` or open an IDE (like Visual Studio/Rider) to compile—they just double-click the `.uproject` file and the installer flawlessly handles the rest.

---

## 2. Packless Recompile Mode
**How it triggers:** Automatically activates if your `config.json` doesn't include any pre-compiled versions in the `availableVersions` array.

**Behavior:**
Changes the main installer UI to a simplified "Plugin Recompiler" screen. If a project is selected (or passed as an argument), it immediately triggers an automatic compilation of the plugin source code.

**Use Case:**
Distributing plugin source and compiling locally against a supported installed engine. Source compatibility and Unreal toolchain requirements still apply; source delivery does not guarantee compatibility with every engine version.

---

## 3. Standard Dashboard Mode
**How it triggers:** Opening the Gorgeous Installer executable normally with no command-line arguments.

**Behavior:**
Opens the full, premium Fyne application dashboard. Users can navigate the sidebar, tweak settings (like Windows `.uproject` registry file associations), browse for Unreal Engine projects on their hard drive, select specific plugin versions to install, and read update changelogs.

**Use Case:**
Locate an Unreal project, inspect its install plan and install the selected payload. Windows also provides per-user self-installation, Start Menu integration and uninstall controls; see [Windows workflows](Windows.md).

---

## 4. Verify-Compatibility Mode (The Rescuer)
**How it triggers:** Via the `--verify-compatibility` CLI flag (which is typically invoked by a native Unreal Engine C++ startup hook if the engine detects stale or mismatched DLLs).

**Behavior:**
Pops up a dedicated warning modal telling the user that the plugin binaries are out-of-sync with their current Engine version, and immediately offers to recompile them to fix the crash.

**Use Case:**
Preventing the dreaded "Missing Modules" Unreal Engine crash. Instead of the user being booted back to the desktop with a confusing error, Gorgeous Installer catches the failure, explains exactly what happened, and offers a beautiful 1-click button to recompile and fix the issue automatically.

---

## 5. Silent CLI Mode
**How it triggers:** Passing arguments like `--cli --action install --project /path/to/project.uproject` from the terminal.

**Behavior:**
Completely disables the graphical UI. It runs installations, unzipping, file transfers, and UnrealBuildTool compilations entirely in the background, printing structured status updates to standard terminal output.

**Use Case:**
Build servers, CI/CD pipelines (like Jenkins or GitHub Actions), or power users who want to automate plugin updates across a massive studio of 50+ developer workstations using automated shell scripts.
