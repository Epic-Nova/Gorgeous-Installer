# Contributing to Gorgeous Installer

Thanks for helping improve the Gorgeous Things installation experience.

## Development

- Install Go and the native Fyne prerequisites for your platform.
- Fork the repository and create a focused branch from `master`.
- Run `go test -tags ci ./...` before submitting changes.
- For changes to GUI behavior, include test coverage where possible and describe manual desktop validation.
- For changes to Unreal installation, test both a fresh project and an existing project with unrelated assets.

## Pull requests

Explain the problem, approach, and compatibility implications. Include exact verification commands and distinguish automated tests from Unreal Editor or Windows UI tests that were not run.

Avoid committing compiled binaries, local credentials, engine installations, or generated build output.

## Reporting issues

Include OS, Unreal Engine version, pack type, installer version/commit, expected and actual results, and sanitized logs. Do not include access tokens, signing keys, or personal project secrets.
