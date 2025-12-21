@echo off
REM LunaBox 构建脚本
REM 用法: build.bat [portable|installer|all]

setlocal enabledelayedexpansion

set "BUILD_MODE=%~1"
if "%BUILD_MODE%"=="" set "BUILD_MODE=all"

REM 获取版本号（可以从 wails.json 或 git tag 获取）
set "VERSION=1.0.0"

REM ldflags 用于注入构建模式
set "LDFLAGS_PORTABLE=-X 'lunabox/internal/utils.buildMode=portable'"
set "LDFLAGS_INSTALLER=-X 'lunabox/internal/utils.buildMode=installer'"

echo ========================================
echo LunaBox Build Script
echo Build Mode: %BUILD_MODE%
echo ========================================
echo.

if "%BUILD_MODE%"=="portable" goto :build_portable
if "%BUILD_MODE%"=="installer" goto :build_installer
if "%BUILD_MODE%"=="all" goto :build_all

echo Unknown build mode: %BUILD_MODE%
echo Usage: build.bat [portable^|installer^|all]
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
echo [1/2] Building Portable Version...
echo ----------------------------------------
wails build -ldflags "%LDFLAGS_PORTABLE%" -o lunabox-portable.exe
if errorlevel 1 (
    echo ERROR: Portable build failed!
    exit /b 1
)
echo Portable build completed: bin\lunabox-portable.exe
echo.

REM 创建便携版压缩包
if exist "bin\lunabox-portable.exe" (
    echo Creating portable ZIP package...
    if exist "bin\LunaBox-Portable-%VERSION%.zip" del "bin\LunaBox-Portable-%VERSION%.zip"
    powershell -Command "Compress-Archive -Path 'bin\lunabox-portable.exe' -DestinationPath 'bin\LunaBox-Portable-%VERSION%.zip'"
    echo Created: bin\LunaBox-Portable-%VERSION%.zip
)
echo.
goto :eof

:build_installer
echo [2/2] Building Installer Version...
echo ----------------------------------------
wails build -ldflags "%LDFLAGS_INSTALLER%" -nsis
if errorlevel 1 (
    echo ERROR: Installer build failed!
    exit /b 1
)
echo Installer build completed!
echo.

REM 重命名安装包以区分版本
if exist "bin\lunabox-amd64-installer.exe" (
    move /Y "bin\lunabox-amd64-installer.exe" "bin\LunaBox-%VERSION%-Setup.exe" >nul
    echo Created: bin\LunaBox-%VERSION%-Setup.exe
)
echo.
goto :eof

:done
echo ========================================
echo Build completed successfully!
echo ========================================
echo.
echo Output files:
echo   - Portable: bin\LunaBox-Portable-%VERSION%.zip
echo   - Installer: bin\LunaBox-%VERSION%-Setup.exe
echo.
echo Portable version: Data stored in program directory
echo Installer version: Data stored in %%APPDATA%%\LunaBox
echo.
endlocal
