<div align="center">

# Gorgeous Installer

### Ship the systems. Skip the setup.

**The native desktop installer and publishing toolkit for Gorgeous Things and Unreal Engine.**

[![Go](https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Fyne](https://img.shields.io/badge/Fyne-Desktop_UI-00AEEF)](https://fyne.io/)
[![Unreal Engine](https://img.shields.io/badge/Unreal_Engine-313131?logo=unrealengine&logoColor=white)](https://www.unrealengine.com/)
[![Issues](https://img.shields.io/github/issues/Epic-Nova/Gorgeous-Installer)](https://github.com/Epic-Nova/Gorgeous-Installer/issues)
[![Stars](https://img.shields.io/github/stars/Epic-Nova/Gorgeous-Installer)](https://github.com/Epic-Nova/Gorgeous-Installer/stargazers)

[Get started](Documentation/QUICKSTART.md) · [Example packs](Documentation/EXAMPLES.md) · [Report an issue](https://github.com/Epic-Nova/Gorgeous-Installer/issues/new/choose) · [Gorgeous Core](https://github.com/Epic-Nova/Gorgeous-Core)

</div>

---

## One installer. Different ways to build.

Gorgeous Installer helps Unreal Engine developers install versioned content and code packs, validate package integrity, and publish standalone installers. Built with **Go + Fyne**, it provides a native GUI and CLI without requiring Gorgeous Core in the target project for standalone examples.

| Install | Publish | Verify |
| :-- | :-- | :-- |
| Detect Unreal Engine versions and choose compatible payloads | Build versioned offline installer bundles | Validate SHA-256 manifests |
| Install, update, or reinstall changed files | Package content-only or complete-project examples | Inspect changes before copying |
| Recompile supported code packs with Unreal build tools | Prepare downloads for distribution | Follow live installation and build logs |

### Pick the right package

| Package | Destination | Purpose |
| --- | --- | --- |
| `content` | Existing plugin's `Content/` | Plugin content |
| `code` | Existing plugin's `Source/` | Plugin source and compilation |
| `hybrid` | Existing plugin root | Combined plugin payload |
| `content-example` | Existing project's `Content/` | Asset-only examples |
| `project-example` | Empty destination directory | Complete sample projects |

## Getting started

**Requirements:** Go and the platform-specific prerequisites for Fyne; Unreal Engine is required for editor integration and code compilation. Asset-only examples with a numeric EngineAssociation can be installed without a local engine installation.

```bash
go build ./...
go test -tags ci ./...
```

To launch the GUI, build the main entry point with your platform's Fyne prerequisites:

```bash
go run ./cmd/main
```

For CLI installation:

```bash
go run ./cmd/main -cli -project "/path/to/Game.uproject"
```

For Windows build and signing instructions, see the [developer guide](.github/DEVELOPMENT.md). For example payload layouts and publishing, see [Examples](Documentation/EXAMPLES.md).

## How it works

```text
Selected .uproject
       |
       v
Engine association + version selection
       |
       v
Embedded package + SHA validation
       |
       v
Install plan (install / update / reinstall)
       |
       v
Copy changed files -> optional Unreal build
```

The installer compares packaged and destination files before applying updates. Content examples preserve unrelated project assets; full-project examples require an empty destination.

## Project structure

| Path | Responsibility |
| --- | --- |
| `cmd/main/` | Installer CLI and GUI entry point |
| `cmd/builder/` | Package configuration helper |
| `internal/config/` | Pack configuration and versions |
| `internal/installer/` | Planning, installation, integrity checks and compilation |
| `internal/unreal/` | Project and engine discovery |
| `internal/ui/` | Fyne desktop interface and publisher |
| `Documentation/` | Quickstart, examples and distribution notes |
| `.github/` | Contribution and issue templates |

## Contributing

Issues and pull requests are welcome. Start with [CONTRIBUTING](.github/CONTRIBUTING.md), use the [issue templates](https://github.com/Epic-Nova/Gorgeous-Installer/issues/new/choose), and include your operating system, Unreal Engine version, and reproducible steps for installer bugs.

## The Gorgeous Things ecosystem

Gorgeous Installer is maintained by [Epic Nova](https://epicnova.net) alongside [Gorgeous Core](https://github.com/Epic-Nova/Gorgeous-Core). It is a delivery tool, not a runtime dependency of Gorgeous Core.

For technical details, see the [documentation index](Documentation/QUICKSTART.md).
