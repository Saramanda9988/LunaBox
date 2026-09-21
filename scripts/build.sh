#!/usr/bin/env bash

# LunaBox Unix release builder for Wails v3.
# Usage: ./scripts/build.sh [installer|appimage|all] [version] [amd64|arm64]

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

BUILD_MODE="${1:-all}"
VERSION_ARG="${2:-}"
TARGET_ARCH="${3:-}"

usage() {
    echo "Usage: ./scripts/build.sh [installer|appimage|all] [version] [amd64|arm64]"
}

case "$BUILD_MODE" in
    installer|appimage|all) ;;
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

if [[ "$HOST_OS" == "Darwin" && "$BUILD_MODE" == "appimage" ]]; then
    echo "ERROR: macOS distribution uses a DMG; AppImage mode is only available on Linux."
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

linux_package_version() {
    local value="$1"
    local prerelease_separator="~"
    printf '%s' "${value/-/$prerelease_separator}"
}

is_semver_like() {
    local value="${1#v}"
    [[ "$value" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z][0-9A-Za-z.-]*)?(\+[0-9A-Za-z][0-9A-Za-z.-]*)?$ ]]
}

git_short_commit() {
    local commit="${GITHUB_SHA:-}"
    if [[ -z "$commit" ]]; then
        commit="$(git rev-parse --short HEAD 2>/dev/null || true)"
    fi
    commit="${commit:0:7}"
    [[ -n "$commit" ]] || commit="unknown"
    printf '%s' "$commit"
}

git_commit_count() {
    local count
    count="$(git rev-list --count HEAD 2>/dev/null || true)"
    [[ -n "$count" ]] || count="0"
    printf '%s' "$count"
}

latest_base_version() {
    local tag base
    tag="$(git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null || true)"
    base="${tag#v}"
    if ! is_semver_like "$base"; then
        base="1.0.0"
    fi
    printf '%s' "$base"
}

resolve_build_version() {
    local value="$1"
    if [[ -n "$value" ]] && is_semver_like "$value"; then
        printf '%s\n' "${value#v}"
        return
    fi

    if [[ -z "$value" ]]; then
        latest_base_version
        return
    fi

    printf '%s-dev.%s+%s\n' "$(latest_base_version)" "$(git_commit_count)" "$(git_short_commit)"
}

read_build_env

VERSION="$(resolve_build_version "$VERSION_ARG")"

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
LDFLAGS_INSTALLER="$LDFLAGS_BASE $(ldflag_set 'lunabox/internal/version.BuildMode' 'installer')"
LDFLAGS_APPIMAGE="$LDFLAGS_BASE $(ldflag_set 'lunabox/internal/version.BuildMode' 'appimage')"

BIN_DIR="build/bin"
APP_BINARY="$BIN_DIR/LunaBox"
CLI_BINARY="$BIN_DIR/lunacli"
APP_BUNDLE="$BIN_DIR/LunaBox.app"
DMG_PATH="$BIN_DIR/LunaBox-${VERSION}-macos-${TARGET_ARCH}.dmg"
DMG_STAGING="build/dmg/LunaBox-${VERSION}-macos-${TARGET_ARCH}"
LINUX_DEB_PATH="$BIN_DIR/LunaBox-${VERSION}-linux-${TARGET_ARCH}.deb"
LINUX_RPM_PATH="$BIN_DIR/LunaBox-${VERSION}-linux-${TARGET_ARCH}.rpm"
LINUX_APPIMAGE_STAGING="build/linux/appimage/LunaBox.AppDir"
LINUX_APPIMAGE_PATH="$BIN_DIR/LunaBox-${VERSION}-linux-${TARGET_ARCH}.AppImage"
LINUX_SEVENZIP_SOURCE="lib/linux${TARGET_ARCH}/7z/7zz"
LINUX_SEVENZIP_PACKAGE_PATH="$BIN_DIR/7zz"
LINUX_INSTALLER_LAUNCHER="$BIN_DIR/LunaBox-linux-launcher"
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
    if [[ "$BUILD_MODE" == "installer" || "$BUILD_MODE" == "all" ]]; then
        check_tool nfpm
    fi
    if [[ "$BUILD_MODE" == "appimage" || "$BUILD_MODE" == "all" ]]; then
        check_tool appimagetool
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
if [[ "$HOST_OS" == "Linux" ]]; then echo "Package Version: $(linux_package_version "$VERSION")"; fi
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

    write_linux_runtime_env() {
        if [[ "$TARGET_ARCH" != "arm64" ]]; then
            return 0
        fi
        cat <<'EOF'
if [ "${LUNABOX_WEBKIT_MODE:-}" != "native" ]; then
    export WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS="${WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS:-1}"
    export WEBKIT_DISABLE_COMPOSITING_MODE="${WEBKIT_DISABLE_COMPOSITING_MODE:-1}"
    export WEBKIT_DISABLE_DMABUF_RENDERER="${WEBKIT_DISABLE_DMABUF_RENDERER:-1}"
    export LIBGL_ALWAYS_SOFTWARE="${LIBGL_ALWAYS_SOFTWARE:-1}"
    export MESA_LOADER_DRIVER_OVERRIDE=llvmpipe
    export GALLIUM_DRIVER=llvmpipe
fi
EOF
    }

    write_linux_installer_launcher() {
        local target="$1"
        mkdir -p "$(dirname "$target")"
        {
            cat <<'EOF'
#!/usr/bin/env sh
set -eu
EOF
            write_linux_runtime_env
            cat <<'EOF'
exec /usr/lib/lunabox/LunaBox "$@"
EOF
        } > "$target"
        chmod 755 "$target"
    }

    write_linux_appimage_apprun() {
        local target="$1"
        mkdir -p "$(dirname "$target")"
        {
            cat <<'EOF'
#!/usr/bin/env sh
set -eu
APP_RUN_PATH="$(readlink -f "$0" 2>/dev/null || printf '%s' "$0")"
APP_DIR="$(CDPATH= cd "$(dirname "$APP_RUN_PATH")" && pwd -P)"
export GTK_A11Y="${GTK_A11Y:-none}"
if [ -n "${APPIMAGE:-}" ]; then
    export LUNABOX_APPIMAGE_PATH="$APPIMAGE"
fi
EOF
            write_linux_runtime_env
            cat <<'EOF'
case "${1:-}" in
    cli|lunacli)
        shift
        exec "$APP_DIR/usr/bin/lunacli" "$@"
        ;;
esac
exec "$APP_DIR/usr/bin/LunaBox" "$@"
EOF
        } > "$target"
        chmod 755 "$target"
    }

    write_linux_appimage_desktop() {
        local target="$1"
        mkdir -p "$(dirname "$target")"
        cat > "$target" <<EOF
[Desktop Entry]
Version=1.0
Type=Application
Name=LunaBox
Comment=LunaBox game library manager
Exec=LunaBox %u
Terminal=false
Icon=io.github.saramanda9988.lunabox
Categories=Game;
StartupWMClass=io.github.saramanda9988.lunabox
MimeType=x-scheme-handler/lunabox;
X-AppImage-Name=LunaBox
X-AppImage-Version=$VERSION
EOF
    }

    linux_appimage_arch() {
        case "$TARGET_ARCH" in
            amd64) printf '%s\n' "x86_64" ;;
            arm64) printf '%s\n' "aarch64" ;;
            *)
                echo "ERROR: Unsupported AppImage target architecture: $TARGET_ARCH" >&2
                exit 1
                ;;
        esac
    }

    if [[ "$BUILD_MODE" == "installer" || "$BUILD_MODE" == "all" ]]; then
        echo "[1/2] Creating Linux deb and rpm packages..."
        build_linux_binaries "$LDFLAGS_INSTALLER"
        rm -f "$LINUX_DEB_PATH" "$LINUX_RPM_PATH"
        write_linux_installer_launcher "$LINUX_INSTALLER_LAUNCHER"
        stage_linux_sevenzip "$LINUX_SEVENZIP_PACKAGE_PATH"
        NFPM_VERSION="$(linux_package_version "$VERSION")"
        GOARCH="$TARGET_ARCH" MAINTAINER="${MAINTAINER:-LunaBox contributors}" VERSION="$NFPM_VERSION" \
            nfpm pkg --config build/linux/nfpm/nfpm.yaml --packager deb --target "$LINUX_DEB_PATH"
        GOARCH="$TARGET_ARCH" MAINTAINER="${MAINTAINER:-LunaBox contributors}" VERSION="$NFPM_VERSION" \
            nfpm pkg --config build/linux/nfpm/nfpm.yaml --packager rpm --target "$LINUX_RPM_PATH"
    fi

    if [[ "$BUILD_MODE" == "appimage" || "$BUILD_MODE" == "all" ]]; then
        echo "[2/2] Creating Linux AppImage package..."
        build_linux_binaries "$LDFLAGS_APPIMAGE"
        rm -rf "$LINUX_APPIMAGE_STAGING"
        rm -f "$LINUX_APPIMAGE_PATH"
        mkdir -p \
            "$LINUX_APPIMAGE_STAGING/usr/bin" \
            "$LINUX_APPIMAGE_STAGING/usr/share/applications" \
            "$LINUX_APPIMAGE_STAGING/usr/share/icons/hicolor/512x512/apps"
        write_linux_appimage_apprun "$LINUX_APPIMAGE_STAGING/AppRun"
        write_linux_appimage_desktop "$LINUX_APPIMAGE_STAGING/io.github.saramanda9988.lunabox.desktop"
        cp "$LINUX_APPIMAGE_STAGING/io.github.saramanda9988.lunabox.desktop" \
            "$LINUX_APPIMAGE_STAGING/usr/share/applications/io.github.saramanda9988.lunabox.desktop"
        cp "$APP_BINARY" "$LINUX_APPIMAGE_STAGING/usr/bin/LunaBox"
        cp "$CLI_BINARY" "$LINUX_APPIMAGE_STAGING/usr/bin/lunacli"
        chmod 755 "$LINUX_APPIMAGE_STAGING/usr/bin/LunaBox" "$LINUX_APPIMAGE_STAGING/usr/bin/lunacli"
        stage_linux_sevenzip "$LINUX_APPIMAGE_STAGING/usr/bin/7zz"
        cp build/appicon.png "$LINUX_APPIMAGE_STAGING/io.github.saramanda9988.lunabox.png"
        cp build/appicon.png "$LINUX_APPIMAGE_STAGING/usr/share/icons/hicolor/512x512/apps/io.github.saramanda9988.lunabox.png"
        APPIMAGE_ARCH="$(linux_appimage_arch)"
        ARCH="$APPIMAGE_ARCH" appimagetool "$LINUX_APPIMAGE_STAGING" "$LINUX_APPIMAGE_PATH"
        chmod 755 "$LINUX_APPIMAGE_PATH"
    fi

    echo
    echo "========================================"
    echo "Build completed successfully."
    if [[ "$BUILD_MODE" == "installer" || "$BUILD_MODE" == "all" ]]; then echo "DEB: $LINUX_DEB_PATH"; fi
    if [[ "$BUILD_MODE" == "installer" || "$BUILD_MODE" == "all" ]]; then echo "RPM: $LINUX_RPM_PATH"; fi
    if [[ "$BUILD_MODE" == "appimage" || "$BUILD_MODE" == "all" ]]; then echo "AppImage: $LINUX_APPIMAGE_PATH"; fi
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
    go build -tags "production,private_mac_apis" -trimpath -buildvcs=false -ldflags "$LDFLAGS_INSTALLER" -o "$APP_BINARY" .
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
