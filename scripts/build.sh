#!/usr/bin/env bash
# Build script for Gorgeous Installer (Native Linux version)

set -e

# --- Configuration & Paths ---
BUILD_DIR="build"
EXE_PATH="$BUILD_DIR/gorgeous-installer"
ICO_PATH="$BUILD_DIR/icon.ico"
META_PATH="$BUILD_DIR/gorgeous-installer.build.json"

# --- Build Environment ---
# Fyne/GUI frameworks require CGO on Linux to link with graphics subsystems (X11/Wayland/OpenGL)
export CGO_ENABLED=1
export CC="gcc"
export CXX="g++"

# --- Argument Parsing ---
CLEAN=false
RUN=false
USE_UPX=false
SKIP_UPX=false

while [[ "$#" -gt 0 ]]; do
    case $1 in
        --Clean) CLEAN=true ;;
        --Run) RUN=true ;;
        --UseUPX) USE_UPX=true ;;
        --SkipUPX) SKIP_UPX=true ;;
        *) echo "Unknown parameter: $1"; exit 1 ;;
    esac
    shift
done

# --- Build Process ---
echo "Building Gorgeous Installer for Linux..."

if [ "$CLEAN" = true ]; then
    echo "Cleaning previous artifacts..."
    rm -rf "$BUILD_DIR"
fi
mkdir -p "$BUILD_DIR"

# 1. SHA Manifest Generation
echo "Generating SHA manifests..."
if [ -f "config.json" ]; then
    VERSION_COUNT=$(jq '.availableVersions | length' config.json)
    for ((i=0; i<$VERSION_COUNT; i++)); do
        PACK_PATH=$(jq -r ".availableVersions[$i].path" config.json)
        SHA_FILE=$(jq -r ".availableVersions[$i].shaFile" config.json)
        
        # Fallback if shaFile is null or empty in config
        if [ "$SHA_FILE" == "null" ] || [ -z "$SHA_FILE" ]; then
            SHA_FILE="${PACK_PATH%.*}.sha256"
        fi
        
        if [ -d "$PACK_PATH" ]; then
            echo "Hashing files in: $PACK_PATH -> $SHA_FILE"
            find "$PACK_PATH" -type f ! -name "*.sha256" | while read -r file; do
                hash=$(sha256sum "$file" | awk '{print $1}')
                rel_path=$(realpath --relative-to="$PACK_PATH" "$file")
                echo "$hash  $rel_path"
            done | sort > "$SHA_FILE"
        else
            echo "Warning: Pack directory '$PACK_PATH' not found. Skipping SHA manifest."
        fi
    done
else
    echo "Warning: config.json not found. Skipping SHA manifest step."
fi

# 2. Compile (Native Linux)
# Note: Added '-v' so you can see every package processing instead of it locking up silently.
echo "Compiling application (Verbose output)..."
GIT_COMMIT=$(git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS="-s -w -buildid= -X gorgeous-installer/internal/buildinfo.CommitSHA=$GIT_COMMIT -X gorgeous-installer/internal/buildinfo.BuildTime=$BUILD_TIME"

go build -v -trimpath -ldflags "$LDFLAGS" -o "$EXE_PATH" ./cmd/main

# 3. Post-Process (UPX Compression)
if [ "$USE_UPX" = true ] || ([ "$SKIP_UPX" = false ] && command -v upx &> /dev/null); then
    echo "Applying UPX size optimization..."
    if command -v upx &> /dev/null; then
        upx --best --lzma "$EXE_PATH"
    else
        echo "Warning: UPX compression skipped because 'upx' utility is not installed."
    fi
fi

# 4. Metadata Generation
echo "Generating build metadata..."
SHA256=$(sha256sum "$EXE_PATH" | awk '{print $1}')
cat <<EOF > "$META_PATH"
{
  "file": "gorgeous-installer",
  "sha256": "$SHA256",
  "commit": "$GIT_COMMIT",
  "buildTimeUtc": "$BUILD_TIME",
  "goVersion": "$(go version)"
}
EOF

echo "Build successful: $EXE_PATH"

if [ "$RUN" = true ]; then
    echo "Launching Gorgeous Installer..."
    ./"$EXE_PATH"
fi