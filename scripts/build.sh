#!/usr/bin/env bash

# LunaBox Unix release builder for Wails v3.
# Usage: ./scripts/build.sh [portable|installer|all] [version] [amd64|arm64]

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

BUILD_MODE="${1:-all}"
VERSION_ARG="${2:-}"
TARGET_ARCH="${3:-}"

usage() {
    echo "Usage: ./scripts/build.sh [portable|installer|all] [version] [amd64|arm64]"
}

case "$BUILD_MODE" in
    portable|installer|all) ;;
    *)
        echo "ERROR: Unknown build mode: $BUILD_MODE"
        usage
        exit 1
        ;;
esac

if [[ "$VERSION_ARG" == "amd64" || "$VERSION_ARG" == "x64" ]]; then
    TARGET_ARCH="amd64"
    VERSION_ARG=""
elif [[ "$VERSION_ARG" == "arm64" || "$VERSION_ARG" == "aarch64" ]]; then
    TARGET_ARCH="arm64"
    VERSION_ARG=""
fi

if [[ -z "$TARGET_ARCH" ]]; then
    case "$(uname -m)" in
        arm64|aarch64) TARGET_ARCH="arm64" ;;
        x86_64|amd64) TARGET_ARCH="amd64" ;;
        *)
            echo "ERROR: Unsupported host architecture: $(uname -m)"
            exit 1
            ;;
    esac
fi

case "$TARGET_ARCH" in
    arm64|aarch64) TARGET_ARCH="arm64" ;;
    amd64|x64|x86_64) TARGET_ARCH="amd64" ;;
    *)
        echo "ERROR: Unsupported target architecture: $TARGET_ARCH"
        usage
        exit 1
        ;;
esac

HOST_OS="$(uname -s)"
case "$HOST_OS" in
    Darwin|Linux) ;;
    *)
        echo "ERROR: scripts/build.sh only supports macOS and Linux hosts."
        exit 1
        ;;
esac

if [[ "$HOST_OS" == "Darwin" && "$BUILD_MODE" == "portable" ]]; then
    echo "ERROR: macOS distribution uses a DMG; portable mode is only available on Linux."
    exit 1
fi

trim_env_value() {
    local value="$1"
    value="$(printf '%s' "$value" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    if [[ "$value" == \"*\" && "$value" == *\" ]]; then
        value="${value:1:${#value}-2}"
    elif [[ "$value" == \'*\' && "$value" == *\' ]]; then
        value="${value:1:${#value}-2}"
    fi
    printf '%s' "$value"
}

BUILD_ENV_FILE=""
if [[ -f ".env.build" ]]; then
    BUILD_ENV_FILE=".env.build"
elif [[ -f ".env" ]]; then
    BUILD_ENV_FILE=".env"
fi

read_build_env() {
    local line key value
    [[ -n "$BUILD_ENV_FILE" ]] || return 0

    while IFS= read -r line || [[ -n "$line" ]]; do
        line="${line%$'\r'}"
        case "$line" in
            ""|\#*) continue ;;
            export\ *) line="${line#export }" ;;
        esac
        [[ "$line" == *=* ]] || continue
        key="$(printf '%s' "${line%%=*}" | xargs)"
        value="$(trim_env_value "${line#*=}")"
        case "$key" in
            LUNABOX_BANGUMI_CLIENT_ID|LUNABOX_BANGUMI_CLIENT_SECRET|LUNABOX_HIKARINAGI_CLIENT_ID|LUNABOX_HIKARINAGI_CLIENT_SECRET|LUNABOX_TOUCHGAL_TOKEN|LUNABOX_UMBRA_CLIENT_ID|LUNABOX_UMBRA_REGISTRATION_TOKEN)
                if [[ -z "${!key:-}" ]]; then
                    printf -v "$key" '%s' "$value"
                    export "$key"
                fi
                ;;
        esac
    done < "$BUILD_ENV_FILE"
}

ldflag_set() {
    local symbol="$1"
    local value="$2"
    if [[ "$value" == *"'"* ]]; then
        echo "ERROR: ldflag value for $symbol contains a single quote." >&2
        exit 1
    fi
    printf -- "-X '%s=%s'" "$symbol" "$value"
}

read_build_env

if [[ -n "$VERSION_ARG" ]]; then
    VERSION="$VERSION_ARG"
else
    VERSION="$(git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null || true)"
    [[ -n "$VERSION" ]] || VERSION="v1.0.0"
fi
VERSION="${VERSION#v}"

GIT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || true)"
[[ -n "$GIT_COMMIT" ]] || GIT_COMMIT="unknown"
BUILD_TIME="$(date '+%Y-%m-%d %H:%M:%S')"

LDFLAGS_BANGUMI=""
BANGUMI_OAUTH_STATUS="disabled"
if [[ -n "${LUNABOX_BANGUMI_CLIENT_ID:-}" ]]; then
    if [[ -z "${LUNABOX_BANGUMI_CLIENT_SECRET:-}" ]]; then
        echo "ERROR: LUNABOX_BANGUMI_CLIENT_ID and LUNABOX_BANGUMI_CLIENT_SECRET must be configured together."
        exit 1
    fi
    LDFLAGS_BANGUMI=" $(ldflag_set 'lunabox/internal/version.BangumiOAuthClientID' "$LUNABOX_BANGUMI_CLIENT_ID") $(ldflag_set 'lunabox/internal/version.BangumiOAuthClientSecret' "$LUNABOX_BANGUMI_CLIENT_SECRET")"
    BANGUMI_OAUTH_STATUS="enabled"
elif [[ -n "${LUNABOX_BANGUMI_CLIENT_SECRET:-}" ]]; then
    echo "ERROR: LUNABOX_BANGUMI_CLIENT_ID and LUNABOX_BANGUMI_CLIENT_SECRET must be configured together."
    exit 1
fi

LDFLAGS_HIKARINAGI=""
HIKARINAGI_OAUTH_STATUS="disabled"
if [[ -n "${LUNABOX_HIKARINAGI_CLIENT_ID:-}" ]]; then
    LDFLAGS_HIKARINAGI=" $(ldflag_set 'lunabox/internal/version.HikarinagiOAuthClientID' "$LUNABOX_HIKARINAGI_CLIENT_ID")"
    if [[ -n "${LUNABOX_HIKARINAGI_CLIENT_SECRET:-}" ]]; then
        LDFLAGS_HIKARINAGI+=" $(ldflag_set 'lunabox/internal/version.HikarinagiOAuthClientSecret' "$LUNABOX_HIKARINAGI_CLIENT_SECRET")"
    fi
    HIKARINAGI_OAUTH_STATUS="enabled"
elif [[ -n "${LUNABOX_HIKARINAGI_CLIENT_SECRET:-}" ]]; then
    echo "ERROR: LUNABOX_HIKARINAGI_CLIENT_SECRET requires LUNABOX_HIKARINAGI_CLIENT_ID."
    exit 1
fi

LDFLAGS_TOUCHGAL=""
TOUCHGAL_TOKEN_STATUS="disabled"
if [[ -n "${LUNABOX_TOUCHGAL_TOKEN:-}" ]]; then
    LDFLAGS_TOUCHGAL=" $(ldflag_set 'lunabox/internal/version.TouchGalAPIToken' "$LUNABOX_TOUCHGAL_TOKEN")"
    TOUCHGAL_TOKEN_STATUS="enabled"
fi

LDFLAGS_UPDATE_SERVICE=""
if [[ -n "${LUNABOX_UPDATE_SERVICE_URL:-}" ]]; then
    LDFLAGS_UPDATE_SERVICE=" $(ldflag_set 'lunabox/internal/version.UpdateServiceURL' "$LUNABOX_UPDATE_SERVICE_URL")"
fi

LDFLAGS_UMBRA=""
UMBRA_REGISTRATION_STATUS="disabled"
if [[ -n "${LUNABOX_UMBRA_CLIENT_ID:-}" ]]; then
    if [[ -z "${LUNABOX_UMBRA_REGISTRATION_TOKEN:-}" ]]; then
        echo "ERROR: LUNABOX_UMBRA_CLIENT_ID and LUNABOX_UMBRA_REGISTRATION_TOKEN must be configured together."
        exit 1
    fi
    LDFLAGS_UMBRA=" $(ldflag_set 'lunabox/internal/version.UmbraOAuthClientID' "$LUNABOX_UMBRA_CLIENT_ID") $(ldflag_set 'lunabox/internal/version.UmbraRegistrationToken' "$LUNABOX_UMBRA_REGISTRATION_TOKEN")"
    UMBRA_REGISTRATION_STATUS="enabled"
elif [[ -n "${LUNABOX_UMBRA_REGISTRATION_TOKEN:-}" ]]; then
    echo "ERROR: LUNABOX_UMBRA_CLIENT_ID and LUNABOX_UMBRA_REGISTRATION_TOKEN must be configured together."
    exit 1
fi

LDFLAGS_BASE="-s -w $(ldflag_set 'lunabox/internal/version.Version' "$VERSION") $(ldflag_set 'lunabox/internal/version.GitCommit' "$GIT_COMMIT") $(ldflag_set 'lunabox/internal/version.BuildTime' "$BUILD_TIME")$LDFLAGS_UPDATE_SERVICE$LDFLAGS_BANGUMI$LDFLAGS_HIKARINAGI$LDFLAGS_TOUCHGAL$LDFLAGS_UMBRA"
LDFLAGS_PORTABLE="$LDFLAGS_BASE $(ldflag_set 'lunabox/internal/version.BuildMode' 'portable')"
LDFLAGS_INSTALLER="$LDFLAGS_BASE $(ldflag_set 'lunabox/internal/version.BuildMode' 'installer')"

BIN_DIR="build/bin"
APP_BINARY="$BIN_DIR/LunaBox"
CLI_BINARY="$BIN_DIR/lunacli"
APP_BUNDLE="$BIN_DIR/LunaBox.app"
DMG_PATH="$BIN_DIR/LunaBox-${VERSION}-macos-${TARGET_ARCH}.dmg"
DMG_STAGING="build/dmg/LunaBox-${VERSION}-macos-${TARGET_ARCH}"
LINUX_PORTABLE_STAGING="build/linux/portable/LunaBox-${VERSION}-linux-${TARGET_ARCH}"
LINUX_PORTABLE_PATH="$BIN_DIR/LunaBox-${VERSION}-linux-${TARGET_ARCH}-portable.tar.gz"
LINUX_DEB_PATH="$BIN_DIR/LunaBox-${VERSION}-linux-${TARGET_ARCH}.deb"
LINUX_RPM_PATH="$BIN_DIR/LunaBox-${VERSION}-linux-${TARGET_ARCH}.rpm"
LINUX_SEVENZIP_SOURCE="lib/linux${TARGET_ARCH}/7z/7zz"
LINUX_SEVENZIP_PACKAGE_PATH="$BIN_DIR/7zz"
# The checked-in 7zz is a universal Mach-O binary (x86_64 + arm64).
MAC_SEVENZIP_SOURCE="lib/macarm64/7z/7zz"

check_tool() {
    command -v "$1" >/dev/null 2>&1 || {
        echo "ERROR: $1 was not found in PATH."
        exit 1
    }
}

check_tool go
check_tool pnpm
check_tool wails3
if [[ "$HOST_OS" == "Darwin" ]]; then
    check_tool hdiutil
    check_tool codesign
else
    check_tool tar
    if [[ "$BUILD_MODE" == "installer" || "$BUILD_MODE" == "all" ]]; then
        check_tool nfpm
    fi
fi

EXPECTED_WAILS_VERSION="$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v3)"
ACTUAL_WAILS_VERSION="$(wails3 version 2>&1)"
if [[ "$EXPECTED_WAILS_VERSION" != "$ACTUAL_WAILS_VERSION" ]]; then
    echo "ERROR: wails3 version mismatch. Expected $EXPECTED_WAILS_VERSION, found $ACTUAL_WAILS_VERSION."
    echo "       Run: go install github.com/wailsapp/wails/v3/cmd/wails3@$EXPECTED_WAILS_VERSION"
    exit 1
fi

if [[ "$HOST_OS" == "Linux" ]]; then
    ./scripts/patch-wails-linux-tray.sh
    if [[ ! -f "$LINUX_SEVENZIP_SOURCE" ]]; then
        echo "ERROR: Missing $LINUX_SEVENZIP_SOURCE"
        exit 1
    fi
fi

echo "========================================"
if [[ "$HOST_OS" == "Linux" ]]; then
    echo "LunaBox Wails v3 Linux Build"
    echo "Target: linux/$TARGET_ARCH"
else
    echo "LunaBox Wails v3 macOS Build"
    echo "Target: darwin/$TARGET_ARCH"
fi
echo "Build Mode: $BUILD_MODE"
echo "Version: $VERSION"
echo "Commit: $GIT_COMMIT"
if [[ -n "$BUILD_ENV_FILE" ]]; then echo "Build Env File: $BUILD_ENV_FILE"; fi
echo "Bangumi OAuth Injection: $BANGUMI_OAUTH_STATUS"
echo "Hikarinagi OAuth Injection: $HIKARINAGI_OAUTH_STATUS"
echo "TouchGAL Token Injection: $TOUCHGAL_TOKEN_STATUS"
echo "Umbra Registration Token Injection: $UMBRA_REGISTRATION_STATUS"
if [[ "$HOST_OS" == "Linux" && -f "$LINUX_SEVENZIP_SOURCE" ]]; then echo "Bundled 7zz: $LINUX_SEVENZIP_SOURCE"; fi
if [[ "$HOST_OS" == "Darwin" && -f "$MAC_SEVENZIP_SOURCE" ]]; then echo "Bundled 7zz: $MAC_SEVENZIP_SOURCE"; fi
echo "========================================"
echo

echo "[prepare] Installing locked frontend dependencies..."
pnpm --dir frontend install --frozen-lockfile

echo "[prepare] Generating Wails v3 bindings..."
wails3 generate bindings -clean=true -ts

echo "[prepare] Building production frontend..."
pnpm --dir frontend build

if [[ "$HOST_OS" == "Linux" ]]; then
    GO_BUILD_TAGS="${GO_BUILD_TAGS:-production}"

    build_linux_binaries() {
        local ldflags="$1"
        echo "[linux] Building GUI and CLI..."
        mkdir -p "$BIN_DIR"
        GOOS=linux GOARCH="$TARGET_ARCH" CGO_ENABLED=1 \
            go build -tags "$GO_BUILD_TAGS" -trimpath -buildvcs=false -ldflags "$ldflags" -o "$APP_BINARY" .
        GOOS=linux GOARCH="$TARGET_ARCH" CGO_ENABLED=1 \
            go build -tags "$GO_BUILD_TAGS" -trimpath -buildvcs=false -ldflags "$ldflags" -o "$CLI_BINARY" ./cmd/lunacli
        chmod 755 "$APP_BINARY" "$CLI_BINARY"
    }

    stage_linux_sevenzip() {
        local target="$1"
        mkdir -p "$(dirname "$target")"
        cp "$LINUX_SEVENZIP_SOURCE" "$target"
        chmod 755 "$target"
    }

    strip_top_level() {
        local path="${1%/}"
        local top_level="${path##*/}"

        case "$top_level" in
            ""|.|..|-*|*/*|*\\*|*$'\n'*|*$'\r'*)
                echo "ERROR: Unsafe archive top-level directory: $top_level" >&2
                exit 1
                ;;
        esac

        printf '%s\n' "$top_level"
    }

    verify_tar_top_level() {
        local archive_path="$1"
        local top_level="$2"
        local entry clean_entry

        while IFS= read -r entry; do
            clean_entry="${entry#./}"
            case "$clean_entry" in
                ""|/*|../*|*/../*|*/..|*$'\n'*|*$'\r'*)
                    echo "ERROR: Unsafe tar entry in $archive_path: $entry" >&2
                    exit 1
                    ;;
                "$top_level"|"$top_level"/*) ;;
                *)
                    echo "ERROR: Tar entry is outside $top_level: $entry" >&2
                    exit 1
                    ;;
            esac
        done < <(tar -tzf "$archive_path")
    }

    if [[ "$BUILD_MODE" == "portable" || "$BUILD_MODE" == "all" ]]; then
        echo "[1/3] Creating Linux portable package..."
        build_linux_binaries "$LDFLAGS_PORTABLE"
        rm -rf "$LINUX_PORTABLE_STAGING"
        rm -f "$LINUX_PORTABLE_PATH"
        mkdir -p "$LINUX_PORTABLE_STAGING"
        cp "$APP_BINARY" "$LINUX_PORTABLE_STAGING/LunaBox"
        cp "$CLI_BINARY" "$LINUX_PORTABLE_STAGING/lunacli"
        cp build/appicon.png "$LINUX_PORTABLE_STAGING/appicon.png"
        stage_linux_sevenzip "$LINUX_PORTABLE_STAGING/bin/7zz"
        portable_top_level="$(strip_top_level "$LINUX_PORTABLE_STAGING")"
        tar -C "$(dirname "$LINUX_PORTABLE_STAGING")" -czf "$LINUX_PORTABLE_PATH" "$portable_top_level"
        verify_tar_top_level "$LINUX_PORTABLE_PATH" "$portable_top_level"
    fi

    if [[ "$BUILD_MODE" == "installer" || "$BUILD_MODE" == "all" ]]; then
        echo "[2/3] Creating Linux deb and rpm packages..."
        build_linux_binaries "$LDFLAGS_INSTALLER"
        rm -f "$LINUX_DEB_PATH" "$LINUX_RPM_PATH"
        stage_linux_sevenzip "$LINUX_SEVENZIP_PACKAGE_PATH"
        export VERSION GOARCH="$TARGET_ARCH" MAINTAINER="${MAINTAINER:-LunaBox contributors}"
        nfpm pkg --config build/linux/nfpm/nfpm.yaml --packager deb --target "$LINUX_DEB_PATH"
        nfpm pkg --config build/linux/nfpm/nfpm.yaml --packager rpm --target "$LINUX_RPM_PATH"
    fi

    echo
    echo "========================================"
    echo "Build completed successfully."
    if [[ "$BUILD_MODE" == "portable" || "$BUILD_MODE" == "all" ]]; then echo "Portable: $LINUX_PORTABLE_PATH"; fi
    if [[ "$BUILD_MODE" == "installer" || "$BUILD_MODE" == "all" ]]; then echo "DEB: $LINUX_DEB_PATH"; fi
    if [[ "$BUILD_MODE" == "installer" || "$BUILD_MODE" == "all" ]]; then echo "RPM: $LINUX_RPM_PATH"; fi
    echo "========================================"
    exit 0
fi

echo "[1/5] Generating macOS icon..."
wails3 generate icons -input build/appicon.png -macfilename build/darwin/icons.icns

echo "[2/5] Building GUI and CLI..."
mkdir -p "$BIN_DIR"
GOOS=darwin GOARCH="$TARGET_ARCH" CGO_ENABLED=1 \
    CGO_CFLAGS="-mmacosx-version-min=12.0" \
    CGO_LDFLAGS="-mmacosx-version-min=12.0" \
    MACOSX_DEPLOYMENT_TARGET="12.0" \
    go build -tags production -trimpath -buildvcs=false -ldflags "$LDFLAGS_INSTALLER" -o "$APP_BINARY" .
GOOS=darwin GOARCH="$TARGET_ARCH" CGO_ENABLED=1 \
    CGO_CFLAGS="-mmacosx-version-min=12.0" \
    CGO_LDFLAGS="-mmacosx-version-min=12.0" \
    MACOSX_DEPLOYMENT_TARGET="12.0" \
    go build -tags production -trimpath -buildvcs=false -ldflags "$LDFLAGS_INSTALLER" -o "$CLI_BINARY" ./cmd/lunacli
chmod 755 "$APP_BINARY" "$CLI_BINARY"

echo "[3/5] Creating app bundle..."
rm -rf "$APP_BUNDLE"
mkdir -p "$APP_BUNDLE/Contents/MacOS" "$APP_BUNDLE/Contents/Resources/bin"
cp "$APP_BINARY" "$APP_BUNDLE/Contents/MacOS/LunaBox"
cp "$CLI_BINARY" "$APP_BUNDLE/Contents/Resources/bin/lunacli"
cp build/darwin/icons.icns "$APP_BUNDLE/Contents/Resources/icons.icns"
cp build/darwin/Info.plist "$APP_BUNDLE/Contents/Info.plist"
chmod 755 "$APP_BUNDLE/Contents/MacOS/LunaBox" "$APP_BUNDLE/Contents/Resources/bin/lunacli"

if [[ -f "$MAC_SEVENZIP_SOURCE" ]]; then
    cp "$MAC_SEVENZIP_SOURCE" "$APP_BUNDLE/Contents/Resources/bin/7zz"
    chmod 755 "$APP_BUNDLE/Contents/Resources/bin/7zz"
fi

echo "[4/5] Signing app bundle..."
if [[ -n "${MACOS_SIGN_IDENTITY:-}" ]]; then
    codesign --force --deep --options runtime --timestamp --sign "$MACOS_SIGN_IDENTITY" "$APP_BUNDLE"
else
    codesign --force --deep --sign - "$APP_BUNDLE"
fi
codesign --verify --deep --strict --verbose=2 "$APP_BUNDLE"

echo "[5/5] Creating DMG..."
rm -rf "$DMG_STAGING"
mkdir -p "$DMG_STAGING"
ditto "$APP_BUNDLE" "$DMG_STAGING/LunaBox.app"
ln -s /Applications "$DMG_STAGING/Applications"
rm -f "$DMG_PATH"

DMG_SOURCE_SIZE_KB="$(du -sk "$DMG_STAGING" | awk '{print $1}')"
if [[ ! "$DMG_SOURCE_SIZE_KB" =~ ^[0-9]+$ ]]; then
    echo "ERROR: Unable to determine the DMG source size."
    exit 1
fi
DMG_SIZE_MB=$(((((DMG_SOURCE_SIZE_KB + 1023) / 1024) * 2) + 64))

echo "DMG source size: ${DMG_SOURCE_SIZE_KB} KiB"
echo "DMG image capacity: ${DMG_SIZE_MB} MiB"
df -h "$BIN_DIR"
hdiutil create \
    -volname "LunaBox" \
    -srcfolder "$DMG_STAGING" \
    -size "${DMG_SIZE_MB}m" \
    -fs HFS+ \
    -ov \
    -format UDZO \
    "$DMG_PATH" >/dev/null
rm -rf "$DMG_STAGING"

if [[ -n "${MACOS_SIGN_IDENTITY:-}" ]]; then
    codesign --force --timestamp --sign "$MACOS_SIGN_IDENTITY" "$DMG_PATH"
    codesign --verify --verbose=2 "$DMG_PATH"
fi

if [[ -n "${MACOS_NOTARY_PROFILE:-}" ]]; then
    xcrun notarytool submit "$DMG_PATH" --keychain-profile "$MACOS_NOTARY_PROFILE" --wait
    xcrun stapler staple "$DMG_PATH"
    xcrun stapler validate "$DMG_PATH"
fi

echo
echo "========================================"
echo "Build completed successfully."
echo "DMG: $DMG_PATH"
echo "App bundle: $APP_BUNDLE"
echo "========================================"
