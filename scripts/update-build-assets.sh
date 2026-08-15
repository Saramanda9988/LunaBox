#!/usr/bin/env bash

# Updates Wails platform metadata while preserving LunaBox's custom Linux assets.
# Usage: bash scripts/update-build-assets.sh <version>

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

VERSION_ARG="${1:-}"

usage() {
    echo "Usage: bash scripts/update-build-assets.sh <version>"
    echo "Example: bash scripts/update-build-assets.sh 1.12.0"
}

if [[ -z "$VERSION_ARG" || $# -ne 1 ]]; then
    usage
    exit 1
fi

VERSION="${VERSION_ARG#v}"
if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "ERROR: Version must use the X.Y.Z format."
    exit 1
fi

command -v wails3 >/dev/null 2>&1 || {
    echo "ERROR: wails3 was not found in PATH."
    exit 1
}

CONFIG_PATH="build/config.yml"
LINUX_DESKTOP_PATH="build/linux/desktop"
LINUX_NFPM_PATH="build/linux/nfpm/nfpm.yaml"

for required_file in "$CONFIG_PATH" "$LINUX_DESKTOP_PATH" "$LINUX_NFPM_PATH"; do
    if [[ ! -f "$required_file" ]]; then
        echo "ERROR: Required file was not found: $required_file"
        exit 1
    fi
done

BACKUP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/lunabox-build-assets.XXXXXX")"
WAILS_UPDATED=0
cp -p "$CONFIG_PATH" "$BACKUP_DIR/config.yml"
cp -p "$LINUX_DESKTOP_PATH" "$BACKUP_DIR/desktop"
cp -p "$LINUX_NFPM_PATH" "$BACKUP_DIR/nfpm.yaml"

restore_linux_assets() {
    cp -p "$BACKUP_DIR/desktop" "$LINUX_DESKTOP_PATH"
    cp -p "$BACKUP_DIR/nfpm.yaml" "$LINUX_NFPM_PATH"
}

cleanup() {
    local exit_code=$?
    trap - EXIT

    if [[ -d "$BACKUP_DIR" ]]; then
        restore_linux_assets || exit_code=1
        if [[ $exit_code -ne 0 && $WAILS_UPDATED -eq 0 ]]; then
            cp -p "$BACKUP_DIR/config.yml" "$CONFIG_PATH" || true
        fi
        rm -rf -- "$BACKUP_DIR"
    fi

    exit "$exit_code"
}
trap cleanup EXIT

UPDATED_CONFIG="$BACKUP_DIR/config.updated.yml"
if ! awk -v version="$VERSION" '
    {
        line = $0
        sub(/\r$/, "", line)

        if (line ~ /^info:[[:space:]]*$/) {
            in_info = 1
            print line
            next
        }
        if (in_info && line ~ /^[^[:space:]]/) {
            in_info = 0
        }
        if (in_info && line ~ /^[[:space:]]+version:[[:space:]]*/) {
            match(line, /^[[:space:]]+/)
            print substr(line, RSTART, RLENGTH) "version: " version
            updated++
            next
        }
        print line
    }
    END {
        if (updated != 1) {
            exit 1
        }
    }
' "$CONFIG_PATH" > "$UPDATED_CONFIG"; then
    echo "ERROR: Unable to update info.version in $CONFIG_PATH."
    exit 1
fi
mv "$UPDATED_CONFIG" "$CONFIG_PATH"

echo "Updating Wails build assets for LunaBox $VERSION..."
wails3 update build-assets \
    -name LunaBox \
    -binaryname LunaBox \
    -config "$CONFIG_PATH" \
    -dir build
WAILS_UPDATED=1

restore_linux_assets
rm -rf -- "$BACKUP_DIR"
trap - EXIT

echo "Updated Wails build assets to version $VERSION."
echo "Preserved $LINUX_DESKTOP_PATH and $LINUX_NFPM_PATH."
