# Windows installation and workflows

Gorgeous Installer supports native Windows builds and publishing. Git Bash, WSL, `cp` and the external `zip` utility are not required for Windows workflows.

## Build and run

Install Go compatible with `go.mod`, Git and a Fyne-compatible C compiler. Unreal C++ compilation also requires the Visual Studio C++ toolchain and Windows SDK supported by your Unreal version. Build prerequisites are checked; build scripts do not install them automatically.

```powershell
.\build.ps1
.\build\gorgeous-installer.exe -gui
```

Windows PowerShell 5.1 and PowerShell 7 can invoke the Windows build. Compiler package output streams live, followed by the compile/link duration. The script also generates the ignored `internal/buildinfo/buildinfo.go` needed by subsequent `go test`, `go run` and direct `go build` commands. Direct builds reuse that metadata until the script regenerates it.

Once metadata exists, CLI-only builds can disable CGO; GUI and smart-card signing code are excluded from that build. For example, set `CGO_ENABLED=0` and build `./cmd/main` with Go. Both Windows and Linux CLI targets were compiled this way; the Windows CLI executable was also run. Native Linux execution remains a separate check.

## Install for the current user

Use the installation controls in Projects/Settings, or run:

```powershell
.\build\gorgeous-installer.exe -install-self
# Optional custom directory:
.\build\gorgeous-installer.exe -install-self -self-install-dir "C:\Tools\Gorgeous Installer"
```

Installation copies the executable into the configured local bin directory and registers a Start Menu shortcut and an entry in Windows **Installed apps**. A custom directory must be absolute. It is per-user, requires no administrator elevation, and does not enable sign-in autostart. Moving the installation to a different directory does not give the old installation permission to remove the new one's registrations.

Uninstall through Windows Installed apps, Settings, or:

```powershell
.\build\gorgeous-installer.exe -uninstall-self
```

Uninstallation removes installer-owned files and registrations. Unrelated files in the selected directory are preserved. If the running installed executable must be removed, a hidden helper waits for that process to exit before deleting it. Close any other running instance that still holds the installed executable.

The installer registers an **Open with** choice for `.uproject` files. Windows may require you to choose it in Default apps; the installer does not alter the protected `UserChoice` key. Association changes are explicit, and restoration preserves a newer user choice.

## Window movement and project selection

The Windows window retains its frameless appearance. Drag the grip strip along the top to move it; window borders provide native resizing and Windows snapping. Controls beneath that strip keep their normal mouse behavior.

Drop one `.uproject` file or a folder containing one project onto the window to select it. Full-project example installers accept an empty destination directory. Invalid, ambiguous and busy-state drops are rejected; dropping a file never starts installation automatically. Browse controls remain available.

## Unreal projects

Installed Windows engines can omit `GenerateProjectFiles.bat`. The installer prefers the shipped `UnrealBuildTool.exe -projectfiles` and retains the batch-file fallback for source engines. Project paths are passed as separate arguments, including paths containing spaces.

Build cancellation terminates only build process trees started by this installer, rather than all `dotnet` or UnrealBuildTool processes on the machine.

Plugin ZIP updates using `--wait-for-pid` wait for the selected Editor process to exit and then continue; the wait does not loop forever after exit. This workflow requires an explicit project and target plugin, as shown in the [CLI guide](CLI_COMMANDS.md).

UnrealBuildTool can still reject generated action paths longer than 260 characters. A fresh compilation initially reproduced this engine constraint in a long temporary directory and succeeded from a shorter directory containing spaces. Keep engine/project roots reasonably short; the installer does not modify engine path-limit checks.

## Publish and update

On Windows, the publisher invokes native `build.ps1` and produces Windows output. Linux uses `build.sh` and can request Windows cross-compilation when its cross-toolchain is installed. ZIP creation and payload copying use Go implementations on both systems. Publish Linux executables from a Linux build environment.

For a binary-update archive shared by multiple operating systems, collect each host's output before publishing: Windows needs `gorgeous-installer.exe`, while Linux needs `gorgeous-installer`. A native Windows build alone is not a multi-OS release archive.

Binary updates require network consent, HTTPS, a matching digest and the configured release-signature trust key. The Windows updater uses a private staging directory, waits for the running installer to exit, replaces/restarts it through a hidden helper and restores the previous executable if replacement or initial startup fails. Source updates locate the installer module rather than writing into the launcher's arbitrary working directory. Developer mode still suppresses automatic source-update checks.

YubiKey signing requires the hardware, its PC/SC support and the appropriate permissions. Failures are reported directly; signing does not kill unrelated agents or restart system services to recover a device.

## Validation

After generating build metadata:

```powershell
go test -tags ci ./...
go vet -tags ci ./...
go test -tags ci -v ./internal/settings ./internal/selfinstall -count=1

# Interactive Windows desktop / graphics driver required for the native drop probe:
$env:GORGEOUS_NATIVE_DROP_TEST = '1'
go test ./internal/ui -run '^$' -v
Remove-Item Env:GORGEOUS_NATIVE_DROP_TEST

$env:GORGEOUS_NATIVE_UPDATE_TEST = '1'
go test -tags ci ./internal/updater -run TestWindowsBinaryUpdateWaitsAndHandlesLiteralPaths -count=1

$env:GORGEOUS_NATIVE_PROCESS_TEST = '1'
go test -tags ci ./internal/unreal -run TestKillUBTDoesNotTargetUnrelatedProcessNames -count=1

$env:GORGEOUS_TEST_ENGINE_PATH = '<UE_ROOT>'
$env:GORGEOUS_TEST_COMPILE_PROJECT = '1'
$env:GORGEOUS_TEST_PROJECT_ROOT = 'C:\Temp\GTTests'
go test -tags ci -v ./internal/unreal -run TestGenerateProjectFilesInstalledEngine -count=1
```

The Windows follow-up includes native checks for Start shortcut/Installed apps registration and uninstall, real window movement/snapping and hit testing, PowerShell build helpers, executable-update/relaunch/rollback and fresh UE 5.7 C++ project generation/compilation. Fixture shell integration uses isolated folders and registry locations. The native file-drop probe sends a valid receiver-owned drop payload through the real GLFW window into the registered project-selection handler; this is distinct from a physical Explorer drag gesture. Physical YubiKey signing, live production publishing and native Linux/macOS execution require separate environments and are not certified by these Windows runs.
