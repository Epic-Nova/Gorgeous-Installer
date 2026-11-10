# Integration Pack Installer Contract

Integration Packs are optional installer executables that extend one or more Gorgeous systems. They are not the same as the website's **Integrates With** relationships, which describe optional interoperability and never imply an installation prerequisite.

## Required package metadata

Each published pack must provide:

- stable pack ID, display name, owner plugin, description, and pack version;
- every extended system ID;
- hard requirements as stable system IDs plus an optional minimum version;
- optional integration IDs separately from hard requirements;
- supported Unreal Engine versions;
- installer executable URL, signature URL, file size, release date, and documentation URL.

Publishing must reject duplicate IDs, unknown system IDs, empty extension targets, a requirement on the pack itself, unsupported engine/version combinations, or an `available` release without both a real installer artifact and its verification metadata.

## Installation behavior

1. Resolve the project and installed-system inventory without changing files.
2. Validate every hard requirement and minimum version before extraction, copying, descriptor edits, or project regeneration.
3. If requirements are satisfied, a relevant system installer may offer or automatically include the pack. Automatic installation must use the same validation and transaction path as manual installation.
4. The website must continue to offer the pack's executable for manual installation.
5. If a requirement is missing, manual or automatic installation must stop before any change and show one clear error listing every missing or incompatible system, its required version, and the detected state.
6. Verification, install, and rollback failures must return a non-zero exit status, preserve the previous project state, and identify the pack and failed stage in the error.

The website catalogue is the discovery surface; the GT Installer remains authoritative for preflight validation, installation, rollback, and actionable errors.
