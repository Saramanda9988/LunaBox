#!/bin/bash
# LunaBox 构建脚本
# 用法: ./build.sh [portable|installer|all]

set -e

BUILD_MODE="${1:-all}"
VERSION="1.0.0"

# ldflags 用于注入构建模式
# -s: strip symbol table, -w: strip DWARF debug info (reduces binary size ~20-30%)
LDFLAGS_PORTABLE="-s -w -X 'lunabox/internal/utils.buildMode=portable'"
LDFLAGS_INSTALLER="-s -w -X 'lunabox/internal/utils.buildMode=installer'"

echo "========================================"
echo "LunaBox Build Script"
echo "Build Mode: $BUILD_MODE"
echo "========================================"
echo

build_portable() {
    echo "[1/2] Building Portable Version..."
    echo "----------------------------------------"
    wails build -ldflags "$LDFLAGS_PORTABLE" -o lunabox-portable
    echo "Portable build completed: bin/lunabox-portable"
    echo
    
    # 创建便携版压缩包
    if [ -f "bin/lunabox-portable" ]; then
        echo "Creating portable archive..."
        cd bin
        tar -czvf "LunaBox-Portable-${VERSION}.tar.gz" lunabox-portable
        cd ..
        echo "Created: bin/LunaBox-Portable-${VERSION}.tar.gz"
    fi
    echo
}

build_installer() {
    echo "[2/2] Building Installer Version..."
    echo "----------------------------------------"
    
    # 检测操作系统
    case "$(uname -s)" in
        Darwin*)
            # macOS - 不支持 NSIS
            wails build -ldflags "$LDFLAGS_INSTALLER"
            echo "macOS build completed (no installer, just app bundle)"
            ;;
        Linux*)
            # Linux - 不支持 NSIS
            wails build -ldflags "$LDFLAGS_INSTALLER"
            echo "Linux build completed"
            ;;
        MINGW*|CYGWIN*|MSYS*)
            # Windows
            wails build -ldflags "$LDFLAGS_INSTALLER" -nsis
            echo "Windows installer build completed"
            ;;
        *)
            echo "Unknown OS, building without installer..."
            wails build -ldflags "$LDFLAGS_INSTALLER"
            ;;
    esac
    echo
}

case "$BUILD_MODE" in
    portable)
        build_portable
        ;;
    installer)
        build_installer
        ;;
    all)
        echo "Building all versions..."
        echo
        build_portable
        build_installer
        ;;
    *)
        echo "Unknown build mode: $BUILD_MODE"
        echo "Usage: ./build.sh [portable|installer|all]"
        exit 1
        ;;
esac

echo "========================================"
echo "Build completed successfully!"
echo "========================================"
echo
echo "Portable version: Data stored in program directory"
echo "Installer version: Data stored in user config directory"
echo
