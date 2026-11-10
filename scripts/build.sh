#!/usr/bin/env bash
# Compatibility entrypoint; retain the legacy switches and stream the canonical build.
set -e
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
args=()
run_after=false
for argument in "$@"; do
    case "$argument" in
        --Clean) args+=(--clean) ;;
        --UseUPX) args+=(--use-upx) ;;
        --SkipUPX) args+=(--skip-upx) ;;
        --Run|--run) run_after=true ;;
        *) args+=("$argument") ;;
    esac
done
bash "$script_dir/../build.sh" "${args[@]}"
if [ "$run_after" = true ]; then
    "$script_dir/../build/gorgeous-installer"
fi
