# 👨‍💻 Gorgeous Installer — Development Guide

Welcome to the **Gorgeous Things** installer development guide! This document covers building, testing, and extending the Go/Fyne desktop application for Unreal Engine content delivery.

> [!NOTE]
> Gorgeous Installer is a **standalone desktop tool**, not an Unreal Engine runtime plugin. You do not need Gorgeous Core to work on the installer. Unreal Engine is needed for editor integration and code-pack compilation tests.

---

## 🚀 Getting started

### 📦 Prerequisites

- **Go:** Install a version compatible with the repository's [go.mod](../go.mod).
- **Git:** For cloning the source and offline publisher builds.
- **Fyne native dependencies:** C compiler, graphics/windowing libraries, and platform-specific development packages. Follow the [official Fyne setup guide](https://docs.fyne.io/started/).
- **Unreal Engine 5:** Required for integration tests involving real projects and compilation; asset-only example installs can work without a locally installed engine when the project uses a numeric engine association.

### 🔽 Clone the repository

```bash
git clone https://github.com/Epic-Nova/Gorgeous-Installer.git
cd Gorgeous-Installer
```

### 🧪 Run automated tests

```bash
go test -tags ci ./...
```

Run the host build script once before these commands: it generates the ignored `internal/buildinfo/buildinfo.go` imported by the application. Direct Go commands reuse that metadata. Windows installation, native interaction checks and optional Unreal integration commands are documented in [Windows workflows](../Documentation/Windows.md).

The `ci` tag uses Fyne's test-friendly configuration where applicable. Automated tests are **not** a substitute for native GUI interaction or opening the resulting assets in Unreal Editor.

### 🖥️ Run the desktop application

```bash
go run ./cmd/main
```

The native GUI requires working Fyne graphics and CGo prerequisites. For automated or headless environments, prefer the supported CLI paths and CI-tagged tests.

---

## 🧰 Platform setup

### 🪟 Windows

Install Go, Git, and a supported C compiler for Fyne. For Unreal Engine C++ builds, install Visual Studio with the **Game development with C++** workload and the Windows SDK.

Build the Windows application using the repository's PowerShell script:

```powershell
.\build.ps1
```

Consult `build.ps1` for available signing, UPX, and SHA validation options.

### 🐧 Linux

Install Go, Git, a C compiler, and Fyne's required OpenGL/X11 development libraries for your distribution. On Linux the offline publisher invokes `build.sh --cross-windows`; Windows cross-compilation requires the appropriate MinGW toolchain and native dependencies. On Windows the publisher invokes native `build.ps1`, without requiring Bash, `zip` or `cp`.

### 🍎 macOS

Install Xcode Command Line Tools, Go, Git, and the platform prerequisites from Fyne. Unreal Engine code-pack builds require a compatible Unreal installation and Xcode setup.

```bash
xcode-select --install
```

---

## 🧩 Repository architecture

| Location | Responsibility |
| --- | --- |
| `cmd/main/` | CLI/GUI entry point and command flags |
| `cmd/builder/` | Standalone pack configuration helper |
| `internal/config/` | Package types and version mappings |
| `internal/installer/` | Install planning, file copying, updates, SHA verification, and compilation |
| `internal/unreal/` | Unreal Engine and `.uproject` discovery |
| `internal/ui/` | Native Fyne UI, example publisher, and installer workflow |
| `Documentation/` | User-facing guides and examples |
| `packs/` | Pack payload roots |

### ⚙️ Installation pipeline

```text
Select .uproject
      ↓
Detect EngineAssociation
      ↓
Select compatible pack version
      ↓
Validate payload / integrity
      ↓
Plan install, update, or reinstall
      ↓
Copy files → optional Unreal build
```

The install plan compares files and identifies changed destinations. Preserve unrelated files and existing project configuration when extending the pipeline.

---

## 📦 Pack types and publishing

| Pack type | Description |
| --- | --- |
| `content` | Installs assets into an existing plugin |
| `code` | Installs source files and recompiles the plugin |
| `hybrid` | Installs a combined plugin payload |
| `content-example` | Copies assets into an existing project's `Content/` |
| `project-example` | Installs a full project into an empty directory |
| `game-feature-example` | Installs a content-only Game Feature Plugin and enables its project plugin entry; runtime activation must be verified in Unreal |

For package folder layouts and publisher usage, see [EXAMPLES.md](../Documentation/EXAMPLES.md). The final pack type is available on its dedicated implementation branch and should be tested against Unreal's Game Features framework before production distribution.

> [!IMPORTANT]
> Installing or enabling a Game Feature Plugin does not itself prove that a `GameFeatureData` asset has activated successfully. Verify plugin discovery, dependencies, and runtime behavior in the target Unreal Engine version.

---

## 🔬 Verification checklist

- [ ] `go test -tags ci ./...` passes.
- [ ] Fresh installation, update, and reinstall produce expected files.
- [ ] Unrelated project assets and plugin settings remain intact.
- [ ] Invalid payloads and mismatched SHA manifests fail safely.
- [ ] Engine version selection works for launcher and source builds.
- [ ] Code-pack compilation works with the target Unreal Engine.
- [ ] Native desktop dialogs and progress reporting work on supported operating systems.
- [ ] Installed examples load correctly in Unreal Editor.
- [ ] Game Feature plugins are discovered and activate as intended, when applicable.

---

## 🤝 Contributing and support

Please read [Contributing](CONTRIBUTING.md), follow the [Code of Conduct](../CODE_OF_CONDUCT.md), and report issues using the [issue forms](https://github.com/Epic-Nova/Gorgeous-Installer/issues/new/choose).

For private security reports, see [SECURITY.md](../SECURITY.md). For general questions, contact [support@epicnova.net](mailto:support@epicnova.net).

**Build the tools that make the next Unreal Engine project easier to ship.**
