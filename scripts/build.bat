@echo off
REM LunaBox Build Script
REM Usage: build.bat [portable|installer|all] [version]

setlocal enabledelayedexpansion

set "BUILD_MODE=%~1"
if "%BUILD_MODE%"=="" set "BUILD_MODE=all"

set "VERSION_ARG=%~2"

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
for /f "tokens=*" %%i in ('powershell -command "Get-Date -Format 'yyyy-MM-dd HH:mm:ss'"') do set "BUILD_TIME=%%i"

REM ldflags for build info injection
REM -s: strip symbol table, -w: strip DWARF debug info (reduces binary size ~20-30%)
set "LDFLAGS_BASE=-s -w -X 'lunabox/internal/version.Version=%VERSION%' -X 'lunabox/internal/version.GitCommit=%GIT_COMMIT%' -X 'lunabox/internal/version.BuildTime=%BUILD_TIME%'"
set "LDFLAGS_PORTABLE=%LDFLAGS_BASE% -X 'lunabox/internal/version.BuildMode=portable'"
set "LDFLAGS_INSTALLER=%LDFLAGS_BASE% -X 'lunabox/internal/version.BuildMode=installer'"

echo ========================================
echo LunaBox Build Script
echo Build Mode: %BUILD_MODE%
echo Version: %VERSION%
echo Commit: %GIT_COMMIT%
echo ========================================
echo.

if "%BUILD_MODE%"=="portable" goto :build_portable
if "%BUILD_MODE%"=="installer" goto :build_installer
if "%BUILD_MODE%"=="all" goto :build_all

echo Unknown build mode: %BUILD_MODE%
echo Usage: build.bat [portable^|installer^|all] [version]
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
wails build -ldflags "%LDFLAGS_PORTABLE%" -o lunabox-portable.exe
if errorlevel 1 (
    echo ERROR: Portable GUI build failed!
    exit /b 1
)
echo Portable GUI build completed: build\bin\lunabox-portable.exe
echo.

echo [2/3] Building CLI Version...
echo ----------------------------------------
go build -trimpath -ldflags "%LDFLAGS_PORTABLE%" -o build\bin\lunabox-cli.exe ./cmd/lunacli
if errorlevel 1 (
    echo ERROR: CLI build failed!
    exit /b 1
)
echo CLI build completed: build\bin\lunabox-cli.exe
echo.

REM Create portable ZIP package with both versions
if exist "build\bin\lunabox-portable.exe" (
    echo [3/3] Creating portable ZIP package...
    set "TEMP_PKG_DIR=build\bin\LunaBox-Portable-%VERSION%"
    if exist "!TEMP_PKG_DIR!" rd /s /q "!TEMP_PKG_DIR!"
    mkdir "!TEMP_PKG_DIR!"
    mkdir "!TEMP_PKG_DIR!\backups"
    mkdir "!TEMP_PKG_DIR!\covers"
    mkdir "!TEMP_PKG_DIR!\backgrounds"
    mkdir "!TEMP_PKG_DIR!\logs"
    
    REM Copy GUI version as LunaBox.exe
    copy "build\bin\lunabox-portable.exe" "!TEMP_PKG_DIR!\LunaBox.exe" >nul
    
    REM Copy CLI version as lunacli.exe
    copy "build\bin\lunabox-cli.exe" "!TEMP_PKG_DIR!\lunacli.exe" >nul
    
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
    
    if exist "build\bin\LunaBox-Portable-%VERSION%.zip" del "build\bin\LunaBox-Portable-%VERSION%.zip"
    powershell -Command "Compress-Archive -Path '!TEMP_PKG_DIR!' -DestinationPath 'build\bin\LunaBox-Portable-%VERSION%.zip'"
    
    REM Clean up temp directory
    rd /s /q "!TEMP_PKG_DIR!"
    
    echo Created: build\bin\LunaBox-Portable-%VERSION%.zip
)
echo.
goto :eof

:build_installer
echo [1/2] Building CLI Version for Installer...
echo ----------------------------------------
go build -trimpath -ldflags "%LDFLAGS_INSTALLER%" -o build\bin\lunacli.exe ./cmd/lunacli
if errorlevel 1 (
    echo ERROR: CLI build for installer failed!
    exit /b 1
)
echo CLI build completed: build\bin\lunacli.exe
echo.

echo [2/2] Building Installer GUI Version...
echo ----------------------------------------
wails build -ldflags "%LDFLAGS_INSTALLER%" -nsis
if errorlevel 1 (
    echo ERROR: Installer GUI build failed!
    exit /b 1
)
echo Installer GUI build completed!
echo.

REM Rename installer to include version
if exist "build\bin\LunaBox-amd64-installer.exe" (
    move /Y "build\bin\LunaBox-amd64-installer.exe" "build\bin\LunaBox-%VERSION%-Setup.exe" >nul
    echo Created: build\bin\LunaBox-%VERSION%-Setup.exe
)
echo.
goto :eof

:done
echo ========================================
echo Build completed successfully!
echo ========================================
echo.
echo Output files:
echo   - Portable: build\bin\LunaBox-Portable-%VERSION%.zip
echo   - Installer: build\bin\LunaBox-%VERSION%-Setup.exe
echo.
echo Portable version: Data stored in program directory
echo Installer version: Data stored in %%APPDATA%%\LunaBox
echo.
endlocal
