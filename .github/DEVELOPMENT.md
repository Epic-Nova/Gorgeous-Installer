# Developer guide

## Architecture

The Go CLI and Fyne GUI share the installer pipeline under `internal/installer`. Pack configuration lives under `internal/config`; Unreal project and engine detection lives under `internal/unreal`.

The install plan compares packaged payload files to destination files and determines install, update, or reinstall. Code and hybrid packs invoke Unreal build tooling; asset-only example packs do not require compilation.

## Common commands

```bash
go test -tags ci ./...
go run ./cmd/main
```

On Windows, use `build.ps1` to produce installer artifacts. The script also supports SHA validation and optional code signing; see the build script for current flags. The offline publisher uses `build.sh --cross-windows` and needs its cross-compilation prerequisites.

## Test checklist

1. Fresh content-pack install.
2. Update with changed and unchanged files.
3. Preserve unrelated destination assets.
4. Validate malformed payload layouts and SHA failures.
5. Test GUI selection of the intended Unreal Engine version.
6. For code packs, verify compilation with the target engine.
7. Test native desktop dialogs and Unreal Editor behavior separately from CI tests.

See [Examples](../Documentation/EXAMPLES.md) for supported standalone package formats.
