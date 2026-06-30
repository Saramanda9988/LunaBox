@echo off
REM LunaBox Build Script
REM Usage: build.bat [portable|installer|installer-payload|installer-package|all] [version] [amd64|arm64]

setlocal enabledelayedexpansion

set "BUILD_ENV_FILE="
if exist ".env.build" set "BUILD_ENV_FILE=.env.build"
if not defined BUILD_ENV_FILE if exist ".env" set "BUILD_ENV_FILE=.env"

if defined BUILD_ENV_FILE (
    for /f "usebackq tokens=1,* delims==" %%A in ("%BUILD_ENV_FILE%") do (
        if /i "%%A"=="LUNABOX_BANGUMI_CLIENT_ID" if not defined LUNABOX_BANGUMI_CLIENT_ID set "LUNABOX_BANGUMI_CLIENT_ID=%%B"
        if /i "%%A"=="LUNABOX_BANGUMI_CLIENT_SECRET" if not defined LUNABOX_BANGUMI_CLIENT_SECRET set "LUNABOX_BANGUMI_CLIENT_SECRET=%%B"
        if /i "%%A"=="LUNABOX_TOUCHGAL_TOKEN" if not defined LUNABOX_TOUCHGAL_TOKEN set "LUNABOX_TOUCHGAL_TOKEN=%%B"
    )
)

set "BANGUMI_CLIENT_ID_RAW=%LUNABOX_BANGUMI_CLIENT_ID%"
set "BANGUMI_CLIENT_SECRET_RAW=%LUNABOX_BANGUMI_CLIENT_SECRET%"
set "TOUCHGAL_TOKEN_RAW=%LUNABOX_TOUCHGAL_TOKEN%"

set "BUILD_MODE=%~1"
if "%BUILD_MODE%"=="" set "BUILD_MODE=all"

set "VERSION_ARG=%~2"
set "TARGET_ARCH=%~3"

if /i "%VERSION_ARG%"=="amd64" (
    set "TARGET_ARCH=amd64"
    set "VERSION_ARG="
)
if /i "%VERSION_ARG%"=="arm64" (
    set "TARGET_ARCH=arm64"
    set "VERSION_ARG="
)
if "%TARGET_ARCH%"=="" set "TARGET_ARCH=amd64"
if /i "%TARGET_ARCH%"=="x64" set "TARGET_ARCH=amd64"
if /i "%TARGET_ARCH%"=="aarch64" set "TARGET_ARCH=arm64"
if /i not "%TARGET_ARCH%"=="amd64" if /i not "%TARGET_ARCH%"=="arm64" (
    echo Unknown target architecture: %TARGET_ARCH%
    echo Usage: build.bat [portable^|installer^|installer-payload^|installer-package^|all] [version] [amd64^|arm64]
    exit /b 1
)

set "WAILS_PLATFORM=windows/%TARGET_ARCH%"
set "GO_BUILD_TAGS="
set "DUCKDB_DLL="
set "DUCKDB_BUILD_LIB_DIR="
set "SEVENZIP_SOURCE_DIR=%CD%\lib\win%TARGET_ARCH%\7z"
set "SEVENZIP_BUILD_DIR=%CD%\build\bin\7z"
if not exist "!SEVENZIP_SOURCE_DIR!\7z.exe" (
    echo ERROR: Missing !SEVENZIP_SOURCE_DIR!\7z.exe
    exit /b 1
)
if not exist "!SEVENZIP_SOURCE_DIR!\7z.dll" (
    echo ERROR: Missing !SEVENZIP_SOURCE_DIR!\7z.dll
    exit /b 1
)

if /i "%TARGET_ARCH%"=="arm64" (
    set "DUCKDB_SOURCE_LIB_DIR=%CD%\lib\winarm64"
    set "DUCKDB_BUILD_LIB_DIR=%CD%\build\duckdb\winarm64"
    set "ARM64_TARGET_TRIPLE=aarch64-w64-windows-gnu"
    set "ARM64_TOOLCHAIN_BIN="
    if not exist "!DUCKDB_SOURCE_LIB_DIR!\duckdb.dll" (
        echo ERROR: Missing !DUCKDB_SOURCE_LIB_DIR!\duckdb.dll
        goto :build_failed
    )
    if not exist "!DUCKDB_SOURCE_LIB_DIR!\duckdb.lib" (
        echo ERROR: Missing !DUCKDB_SOURCE_LIB_DIR!\duckdb.lib
        goto :build_failed
    )
    if not exist "!DUCKDB_BUILD_LIB_DIR!" mkdir "!DUCKDB_BUILD_LIB_DIR!"
    copy /Y "!DUCKDB_SOURCE_LIB_DIR!\duckdb.dll" "!DUCKDB_BUILD_LIB_DIR!\duckdb.dll" >nul
    copy /Y "!DUCKDB_SOURCE_LIB_DIR!\duckdb.lib" "!DUCKDB_BUILD_LIB_DIR!\libduckdb.dll.a" >nul
    set "CGO_ENABLED=1"

    if defined MSYS2_LOCATION if exist "!MSYS2_LOCATION!\clangarm64\bin\clang.exe" set "ARM64_TOOLCHAIN_BIN=!MSYS2_LOCATION!\clangarm64\bin"
    if not defined ARM64_TOOLCHAIN_BIN if exist "C:\msys64\clangarm64\bin\clang.exe" set "ARM64_TOOLCHAIN_BIN=C:\msys64\clangarm64\bin"
    if defined ARM64_TOOLCHAIN_BIN (
        set "CC=!ARM64_TOOLCHAIN_BIN!\clang.exe --target=!ARM64_TARGET_TRIPLE!"
        if exist "!ARM64_TOOLCHAIN_BIN!\clang++.exe" set "CXX=!ARM64_TOOLCHAIN_BIN!\clang++.exe --target=!ARM64_TARGET_TRIPLE!"
    ) else if not defined CC (
        where aarch64-w64-mingw32-gcc >nul 2>nul
        if not errorlevel 1 set "CC=aarch64-w64-mingw32-gcc"
    )
    if not defined CC (
        echo ERROR: Windows ARM64 CGO build requires an ARM64 C compiler.
        echo        Run this script from an ARM64 MSYS2 CLANGARM64 environment or set CC.
        goto :build_failed
    )
    set "ARM64_CC_TARGET="
    for /f "delims=" %%i in ('!CC! -dumpmachine 2^>nul') do if not defined ARM64_CC_TARGET set "ARM64_CC_TARGET=%%i"
    if not defined ARM64_CC_TARGET (
        echo ERROR: Failed to inspect ARM64 C compiler target: !CC!
        goto :build_failed
    )
    echo !ARM64_CC_TARGET! | findstr /i "aarch64 arm64" >nul
    if errorlevel 1 (
        echo ERROR: ARM64 CGO compiler target is not ARM64: !ARM64_CC_TARGET!
        echo        CC=!CC!
        goto :build_failed
    )
    echo ARM64 CGO compiler: !CC! ^(!ARM64_CC_TARGET!^)
    set "CGO_LDFLAGS=-L!DUCKDB_BUILD_LIB_DIR! -lduckdb"
    set "GO_BUILD_TAGS=-tags duckdb_use_lib"
    set "DUCKDB_DLL=!DUCKDB_BUILD_LIB_DIR!\duckdb.dll"
    if defined ARM64_TOOLCHAIN_BIN (
        set "PATH=!ARM64_TOOLCHAIN_BIN!;!DUCKDB_BUILD_LIB_DIR!;!PATH!"
    ) else (
        set "PATH=!DUCKDB_BUILD_LIB_DIR!;!PATH!"
    )
)

if not "%VERSION_ARG%"=="" (
    set "VERSION=%VERSION_ARG%"
) else (
    REM Get version from git tag, fallback to default
    for /f "delims=" %%i in ('git describe --tags --abbrev=0 2^>nul') do set "GIT_VERSION=%%i"
    if not defined GIT_VERSION set "GIT_VERSION=v1.0.0"
    set "VERSION=!GIT_VERSION!"
)

REM Remove 'v' prefix if exists
if "!VERSION:~0,1!"=="v" set "VERSION=!VERSION:~1!"

REM Get Git Commit Hash
for /f "delims=" %%i in ('git rev-parse --short HEAD 2^>nul') do set "GIT_COMMIT=%%i"
if not defined GIT_COMMIT set "GIT_COMMIT=unknown"

REM Get build time
for /f "tokens=*" %%i in ('powershell -NoProfile -command "Get-Date -Format 'yyyy-MM-dd HH:mm:ss'"') do set "BUILD_TIME=%%i"

set "LDFLAGS_BANGUMI="
set "BANGUMI_OAUTH_STATUS=disabled"
if defined BANGUMI_CLIENT_ID_RAW (
    if not defined BANGUMI_CLIENT_SECRET_RAW (
        echo ERROR: LUNABOX_BANGUMI_CLIENT_SECRET is missing.
        exit /b 1
    )
    set "LDFLAGS_BANGUMI= -X 'lunabox/internal/version.BangumiOAuthClientID=!BANGUMI_CLIENT_ID_RAW!' -X 'lunabox/internal/version.BangumiOAuthClientSecret=!BANGUMI_CLIENT_SECRET_RAW!'"
    set "BANGUMI_OAUTH_STATUS=enabled"
)
if not defined BANGUMI_CLIENT_ID_RAW (
    if defined BANGUMI_CLIENT_SECRET_RAW (
        echo ERROR: LUNABOX_BANGUMI_CLIENT_ID is missing.
        exit /b 1
    )
)

set "LDFLAGS_TOUCHGAL="
set "TOUCHGAL_TOKEN_STATUS=disabled"
if defined TOUCHGAL_TOKEN_RAW (
    set "LDFLAGS_TOUCHGAL= -X 'lunabox/internal/version.TouchGalAPIToken=!TOUCHGAL_TOKEN_RAW!'"
    set "TOUCHGAL_TOKEN_STATUS=enabled"
)

REM ldflags for build info injection
REM -s: strip symbol table, -w: strip DWARF debug info (reduces binary size ~20-30%)
set "LDFLAGS_BASE=-s -w -X 'lunabox/internal/version.Version=%VERSION%' -X 'lunabox/internal/version.GitCommit=%GIT_COMMIT%' -X 'lunabox/internal/version.BuildTime=%BUILD_TIME%'!LDFLAGS_BANGUMI!!LDFLAGS_TOUCHGAL!"
set "LDFLAGS_PORTABLE=%LDFLAGS_BASE% -X 'lunabox/internal/version.BuildMode=portable'"
set "LDFLAGS_INSTALLER=%LDFLAGS_BASE% -X 'lunabox/internal/version.BuildMode=installer'"

echo ========================================
echo LunaBox Build Script
echo Build Mode: %BUILD_MODE%
echo Target Arch: %TARGET_ARCH%
echo Version: %VERSION%
echo Commit: %GIT_COMMIT%
if defined BUILD_ENV_FILE echo Build Env File: %BUILD_ENV_FILE%
echo Bangumi OAuth Injection: !BANGUMI_OAUTH_STATUS!
echo TouchGAL Token Injection: !TOUCHGAL_TOKEN_STATUS!
if defined DUCKDB_DLL echo DuckDB Dynamic DLL: !DUCKDB_DLL!
if exist "!SEVENZIP_SOURCE_DIR!\7z.exe" echo Bundled 7z: !SEVENZIP_SOURCE_DIR!
echo ========================================
echo.

if "%BUILD_MODE%"=="portable" goto :build_portable
if "%BUILD_MODE%"=="installer" goto :build_installer
if "%BUILD_MODE%"=="installer-payload" goto :build_installer_payload
if "%BUILD_MODE%"=="installer-package" goto :build_installer_package
if "%BUILD_MODE%"=="all" goto :build_all

echo Unknown build mode: %BUILD_MODE%
echo Usage: build.bat [portable^|installer^|installer-payload^|installer-package^|all] [version] [amd64^|arm64]
exit /b 1

:build_all
echo Building all versions...
echo.
call :build_portable
if errorlevel 1 exit /b 1
call :build_installer
if errorlevel 1 exit /b 1
goto :done

:build_portable
echo [1/3] Building Portable GUI Version...
echo ----------------------------------------
wails build -platform "%WAILS_PLATFORM%" %GO_BUILD_TAGS% -ldflags "%LDFLAGS_PORTABLE%" -o lunabox-%TARGET_ARCH%-portable.exe
if errorlevel 1 (
    echo ERROR: Portable GUI build failed!
    exit /b 1
)
echo Portable GUI build completed: build\bin\lunabox-%TARGET_ARCH%-portable.exe
echo.

echo [2/3] Building CLI Version...
echo ----------------------------------------
set "GOOS=windows"
set "GOARCH=%TARGET_ARCH%"
go build %GO_BUILD_TAGS% -trimpath -ldflags "%LDFLAGS_PORTABLE%" -o build\bin\lunabox-cli.exe ./cmd/lunacli
if errorlevel 1 (
    echo ERROR: CLI build failed!
    exit /b 1
)
echo CLI build completed: build\bin\lunabox-cli.exe
echo.

REM Create portable ZIP package with both versions
if exist "build\bin\lunabox-%TARGET_ARCH%-portable.exe" (
    echo [3/3] Creating portable ZIP package...
    set "TEMP_PKG_DIR=build\bin\LunaBox-%VERSION%-windows-%TARGET_ARCH%-portable"
    if exist "!TEMP_PKG_DIR!" rd /s /q "!TEMP_PKG_DIR!"
    mkdir "!TEMP_PKG_DIR!"
    mkdir "!TEMP_PKG_DIR!\backups"
    mkdir "!TEMP_PKG_DIR!\covers"
    mkdir "!TEMP_PKG_DIR!\backgrounds"
    mkdir "!TEMP_PKG_DIR!\logs"
    
    REM Copy GUI version as LunaBox.exe
    copy "build\bin\lunabox-%TARGET_ARCH%-portable.exe" "!TEMP_PKG_DIR!\LunaBox.exe" >nul
    
    REM Copy CLI version as lunacli.exe
    copy "build\bin\lunabox-cli.exe" "!TEMP_PKG_DIR!\lunacli.exe" >nul

    if defined DUCKDB_DLL copy "!DUCKDB_DLL!" "!TEMP_PKG_DIR!\duckdb.dll" >nul
    if exist "!SEVENZIP_SOURCE_DIR!\7z.exe" (
        mkdir "!TEMP_PKG_DIR!\7z"
        copy /Y "!SEVENZIP_SOURCE_DIR!\7z.exe" "!TEMP_PKG_DIR!\7z\7z.exe" >nul
        if exist "!SEVENZIP_SOURCE_DIR!\7z.dll" copy /Y "!SEVENZIP_SOURCE_DIR!\7z.dll" "!TEMP_PKG_DIR!\7z\7z.dll" >nul
    )
    
    REM Create README
    echo LunaBox Portable v%VERSION% > "!TEMP_PKG_DIR!\README.txt"
    echo. >> "!TEMP_PKG_DIR!\README.txt"
    echo This package contains: >> "!TEMP_PKG_DIR!\README.txt"
    echo   - LunaBox.exe  : GUI version (Double-click to launch) >> "!TEMP_PKG_DIR!\README.txt"
    echo   - lunacli.exe  : CLI version (Use in terminal) >> "!TEMP_PKG_DIR!\README.txt"
    echo. >> "!TEMP_PKG_DIR!\README.txt"
    echo CLI Usage: >> "!TEMP_PKG_DIR!\README.txt"
    echo   lunacli list >> "!TEMP_PKG_DIR!\README.txt"
    echo   lunacli start ^<game-id^> >> "!TEMP_PKG_DIR!\README.txt"
    echo   lunacli protocol register >> "!TEMP_PKG_DIR!\README.txt"
    echo   lunacli protocol unregister >> "!TEMP_PKG_DIR!\README.txt"
    echo   lunacli help >> "!TEMP_PKG_DIR!\README.txt"
    
    if exist "build\bin\LunaBox-%VERSION%-windows-%TARGET_ARCH%-portable.zip" del "build\bin\LunaBox-%VERSION%-windows-%TARGET_ARCH%-portable.zip"
    powershell -NoProfile -Command "Compress-Archive -Path '!TEMP_PKG_DIR!' -DestinationPath 'build\bin\LunaBox-%VERSION%-windows-%TARGET_ARCH%-portable.zip'"
    
    REM Clean up temp directory
    rd /s /q "!TEMP_PKG_DIR!"
    
    echo Created: build\bin\LunaBox-%VERSION%-windows-%TARGET_ARCH%-portable.zip
)
echo.
goto :eof

:prepare_installer_runtime
if exist "build\bin\duckdb.dll" del "build\bin\duckdb.dll"
if defined DUCKDB_DLL copy "!DUCKDB_DLL!" "build\bin\duckdb.dll" >nul
if exist "!SEVENZIP_BUILD_DIR!" rd /s /q "!SEVENZIP_BUILD_DIR!"
if exist "!SEVENZIP_SOURCE_DIR!\7z.exe" (
    mkdir "!SEVENZIP_BUILD_DIR!"
    copy /Y "!SEVENZIP_SOURCE_DIR!\7z.exe" "!SEVENZIP_BUILD_DIR!\7z.exe" >nul
    if exist "!SEVENZIP_SOURCE_DIR!\7z.dll" copy /Y "!SEVENZIP_SOURCE_DIR!\7z.dll" "!SEVENZIP_BUILD_DIR!\7z.dll" >nul
)
goto :eof

:build_installer
call :build_installer_payload
if errorlevel 1 exit /b 1
call :build_installer_package
if errorlevel 1 exit /b 1
goto :eof

:build_installer_payload
echo [1/3] Building CLI Version for Installer...
echo ----------------------------------------
set "GOOS=windows"
set "GOARCH=%TARGET_ARCH%"
go build %GO_BUILD_TAGS% -trimpath -ldflags "%LDFLAGS_INSTALLER%" -o build\bin\lunacli.exe ./cmd/lunacli
if errorlevel 1 (
    echo ERROR: CLI build for installer failed!
    exit /b 1
)
echo CLI build completed: build\bin\lunacli.exe
echo.

echo [2/3] Building Installer GUI Payload...
echo ----------------------------------------
wails build -platform "%WAILS_PLATFORM%" %GO_BUILD_TAGS% -ldflags "%LDFLAGS_INSTALLER%" -o LunaBox.exe
if errorlevel 1 (
    echo ERROR: Installer GUI payload build failed!
    exit /b 1
)
if not exist "build\bin\LunaBox.exe" (
    echo ERROR: Installer GUI payload not found: build\bin\LunaBox.exe
    exit /b 1
)
call :prepare_installer_runtime
if errorlevel 1 exit /b 1
echo Installer GUI payload completed: build\bin\LunaBox.exe
echo.

echo [3/3] Creating installer payload ZIP for signing...
echo ----------------------------------------
set "INSTALLER_PAYLOAD_ZIP=build\bin\LunaBox-%VERSION%-windows-%TARGET_ARCH%-installer-payload.zip"
if exist "!INSTALLER_PAYLOAD_ZIP!" del "!INSTALLER_PAYLOAD_ZIP!"
powershell -NoProfile -Command "Compress-Archive -Path 'build\bin\LunaBox.exe','build\bin\lunacli.exe' -DestinationPath '!INSTALLER_PAYLOAD_ZIP!'"
if errorlevel 1 (
    echo ERROR: Installer payload ZIP creation failed!
    exit /b 1
)
echo Created: !INSTALLER_PAYLOAD_ZIP!
echo.
goto :eof

:build_installer_package
echo [1/1] Building NSIS Installer from Installer Payload...
echo ----------------------------------------
if not exist "build\bin\LunaBox.exe" (
    echo ERROR: Missing signed installer GUI payload: build\bin\LunaBox.exe
    exit /b 1
)
if not exist "build\bin\lunacli.exe" (
    echo ERROR: Missing signed installer CLI payload: build\bin\lunacli.exe
    exit /b 1
)
call :prepare_installer_runtime
if errorlevel 1 exit /b 1

set "WAILS_BINARY_DEFINE=ARG_WAILS_AMD64_BINARY=..\..\bin\LunaBox.exe"
if /i "%TARGET_ARCH%"=="arm64" set "WAILS_BINARY_DEFINE=ARG_WAILS_ARM64_BINARY=..\..\bin\LunaBox.exe"

pushd build\windows\installer
makensis /D%WAILS_BINARY_DEFINE% project.nsi
set "MAKENSIS_EXIT=%ERRORLEVEL%"
popd
if not "%MAKENSIS_EXIT%"=="0" (
    echo ERROR: NSIS installer build failed!
    exit /b %MAKENSIS_EXIT%
)

if exist "build\bin\LunaBox-%VERSION%-windows-%TARGET_ARCH%-setup.exe" del "build\bin\LunaBox-%VERSION%-windows-%TARGET_ARCH%-setup.exe"
if exist "build\bin\LunaBox-%TARGET_ARCH%-installer.exe" (
    move /Y "build\bin\LunaBox-%TARGET_ARCH%-installer.exe" "build\bin\LunaBox-%VERSION%-windows-%TARGET_ARCH%-setup.exe" >nul
) else if exist "build\bin\lunabox-%TARGET_ARCH%-installer.exe" (
    move /Y "build\bin\lunabox-%TARGET_ARCH%-installer.exe" "build\bin\LunaBox-%VERSION%-windows-%TARGET_ARCH%-setup.exe" >nul
) else (
    echo ERROR: Installer not found for %TARGET_ARCH%.
    exit /b 1
)
echo Created: build\bin\LunaBox-%VERSION%-windows-%TARGET_ARCH%-setup.exe
echo.
goto :eof

:done
echo ========================================
echo Build completed successfully!
echo ========================================
echo.
echo Output files:
echo   - Portable: build\bin\LunaBox-%VERSION%-windows-%TARGET_ARCH%-portable.zip
echo   - Installer: build\bin\LunaBox-%VERSION%-windows-%TARGET_ARCH%-setup.exe
echo.
echo Portable version: Data stored in program directory
echo Installer version: Data stored in %%APPDATA%%\LunaBox
echo.
endlocal
exit /b 0

:build_failed
endlocal
exit /b 1
