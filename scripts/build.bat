@echo off
REM LunaBox Build Script
REM Usage: build.bat [portable|installer|all] [version] [amd64|arm64]

setlocal enabledelayedexpansion

set "BUILD_ENV_FILE="
if exist ".env.build" set "BUILD_ENV_FILE=.env.build"
if not defined BUILD_ENV_FILE if exist ".env" set "BUILD_ENV_FILE=.env"

if defined BUILD_ENV_FILE (
    for /f "usebackq tokens=1,* delims==" %%A in ("%BUILD_ENV_FILE%") do (
        if /i "%%A"=="LUNABOX_BANGUMI_CLIENT_ID" if not defined LUNABOX_BANGUMI_CLIENT_ID set "LUNABOX_BANGUMI_CLIENT_ID=%%B"
        if /i "%%A"=="LUNABOX_BANGUMI_CLIENT_SECRET" if not defined LUNABOX_BANGUMI_CLIENT_SECRET set "LUNABOX_BANGUMI_CLIENT_SECRET=%%B"
    )
)

set "BANGUMI_CLIENT_ID_RAW=%LUNABOX_BANGUMI_CLIENT_ID%"
set "BANGUMI_CLIENT_SECRET_RAW=%LUNABOX_BANGUMI_CLIENT_SECRET%"

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
    echo Usage: build.bat [portable^|installer^|all] [version] [amd64^|arm64]
    exit /b 1
)

set "WAILS_PLATFORM=windows/%TARGET_ARCH%"
set "GO_BUILD_TAGS="
set "DUCKDB_DLL="
set "DUCKDB_BUILD_LIB_DIR="

if /i "%TARGET_ARCH%"=="arm64" (
    set "DUCKDB_SOURCE_LIB_DIR=%CD%\lib\winarm64"
    set "DUCKDB_BUILD_LIB_DIR=%CD%\build\duckdb\winarm64"
    if not exist "!DUCKDB_SOURCE_LIB_DIR!\duckdb.dll" (
        echo ERROR: Missing !DUCKDB_SOURCE_LIB_DIR!\duckdb.dll
        exit /b 1
    )
    if not exist "!DUCKDB_SOURCE_LIB_DIR!\duckdb.lib" (
        echo ERROR: Missing !DUCKDB_SOURCE_LIB_DIR!\duckdb.lib
        exit /b 1
    )
    if not exist "!DUCKDB_BUILD_LIB_DIR!" mkdir "!DUCKDB_BUILD_LIB_DIR!"
    copy /Y "!DUCKDB_SOURCE_LIB_DIR!\duckdb.dll" "!DUCKDB_BUILD_LIB_DIR!\duckdb.dll" >nul
    copy /Y "!DUCKDB_SOURCE_LIB_DIR!\duckdb.lib" "!DUCKDB_BUILD_LIB_DIR!\libduckdb.dll.a" >nul
    set "CGO_ENABLED=1"
    if not defined CC (
        if exist "C:\msys64\clangarm64\bin\clang.exe" (
            set "CC=C:\msys64\clangarm64\bin\clang.exe --target=aarch64-w64-windows-gnu"
            if not defined CXX if exist "C:\msys64\clangarm64\bin\clang++.exe" set "CXX=C:\msys64\clangarm64\bin\clang++.exe --target=aarch64-w64-windows-gnu"
        ) else (
            where aarch64-w64-mingw32-gcc >nul 2>nul
            if not errorlevel 1 set "CC=aarch64-w64-mingw32-gcc"
        )
    )
    if not defined CC (
        echo ERROR: Windows ARM64 CGO build requires an ARM64 C compiler.
        echo        Run this script from an ARM64 MSYS2 CLANGARM64 environment or set CC.
        exit /b 1
    )
    set "CGO_LDFLAGS=-L!DUCKDB_BUILD_LIB_DIR! -lduckdb"
    set "GO_BUILD_TAGS=-tags duckdb_use_lib"
    set "DUCKDB_DLL=!DUCKDB_BUILD_LIB_DIR!\duckdb.dll"
    set "PATH=C:\msys64\clangarm64\bin;!DUCKDB_BUILD_LIB_DIR!;!PATH!"
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

REM ldflags for build info injection
REM -s: strip symbol table, -w: strip DWARF debug info (reduces binary size ~20-30%)
set "LDFLAGS_BASE=-s -w -X 'lunabox/internal/version.Version=%VERSION%' -X 'lunabox/internal/version.GitCommit=%GIT_COMMIT%' -X 'lunabox/internal/version.BuildTime=%BUILD_TIME%'!LDFLAGS_BANGUMI!"
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
if defined DUCKDB_DLL echo DuckDB Dynamic DLL: !DUCKDB_DLL!
echo ========================================
echo.

if "%BUILD_MODE%"=="portable" goto :build_portable
if "%BUILD_MODE%"=="installer" goto :build_installer
if "%BUILD_MODE%"=="all" goto :build_all

echo Unknown build mode: %BUILD_MODE%
echo Usage: build.bat [portable^|installer^|all] [version] [amd64^|arm64]
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

:build_installer
echo [1/2] Building CLI Version for Installer...
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

echo [2/2] Building Installer GUI Version...
echo ----------------------------------------
if exist "build\bin\duckdb.dll" del "build\bin\duckdb.dll"
if defined DUCKDB_DLL copy "!DUCKDB_DLL!" "build\bin\duckdb.dll" >nul
wails build -platform "%WAILS_PLATFORM%" %GO_BUILD_TAGS% -ldflags "%LDFLAGS_INSTALLER%" -nsis
if errorlevel 1 (
    echo ERROR: Installer GUI build failed!
    exit /b 1
)
echo Installer GUI build completed!
echo.

REM Rename installer to include version
if exist "build\bin\LunaBox-%TARGET_ARCH%-installer.exe" (
    move /Y "build\bin\LunaBox-%TARGET_ARCH%-installer.exe" "build\bin\LunaBox-%VERSION%-windows-%TARGET_ARCH%-setup.exe" >nul
    echo Created: build\bin\LunaBox-%VERSION%-windows-%TARGET_ARCH%-setup.exe
)
if exist "build\bin\lunabox-%TARGET_ARCH%-installer.exe" (
    move /Y "build\bin\lunabox-%TARGET_ARCH%-installer.exe" "build\bin\LunaBox-%VERSION%-windows-%TARGET_ARCH%-setup.exe" >nul
    echo Created: build\bin\LunaBox-%VERSION%-windows-%TARGET_ARCH%-setup.exe
)
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
