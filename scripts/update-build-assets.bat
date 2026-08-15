@echo off
setlocal EnableExtensions DisableDelayedExpansion

REM Updates Wails platform metadata while preserving LunaBox's custom Linux assets.
REM Usage: scripts\update-build-assets.bat <version>

cd /d "%~dp0\.."

set "VERSION_ARG=%~1"
if "%VERSION_ARG%"=="" goto :usage
if not "%~2"=="" goto :usage

set "VERSION=%VERSION_ARG%"
if /i "%VERSION:~0,1%"=="v" set "VERSION=%VERSION:~1%"
set "LUNABOX_ASSET_VERSION=%VERSION%"
powershell -NoProfile -Command "if ($env:LUNABOX_ASSET_VERSION -notmatch '^\d+\.\d+\.\d+$') { exit 1 }"
if errorlevel 1 (
    echo ERROR: Version must use the X.Y.Z format.
    exit /b 1
)

where wails3 >nul 2>nul
if errorlevel 1 (
    echo ERROR: wails3 was not found in PATH.
    exit /b 1
)

set "CONFIG_PATH=build\config.yml"
set "LINUX_DESKTOP_PATH=build\linux\desktop"
set "LINUX_NFPM_PATH=build\linux\nfpm\nfpm.yaml"

if not exist "%CONFIG_PATH%" (
    echo ERROR: Required file was not found: %CONFIG_PATH%
    exit /b 1
)
if not exist "%LINUX_DESKTOP_PATH%" (
    echo ERROR: Required file was not found: %LINUX_DESKTOP_PATH%
    exit /b 1
)
if not exist "%LINUX_NFPM_PATH%" (
    echo ERROR: Required file was not found: %LINUX_NFPM_PATH%
    exit /b 1
)

set "BACKUP_DIR=%TEMP%\lunabox-build-assets-%RANDOM%-%RANDOM%"
mkdir "%BACKUP_DIR%" >nul 2>nul
if errorlevel 1 (
    echo ERROR: Unable to create temporary directory: %BACKUP_DIR%
    exit /b 1
)

copy /y "%CONFIG_PATH%" "%BACKUP_DIR%\config.yml" >nul
if errorlevel 1 goto :backup_failed
copy /y "%LINUX_DESKTOP_PATH%" "%BACKUP_DIR%\desktop" >nul
if errorlevel 1 goto :backup_failed
copy /y "%LINUX_NFPM_PATH%" "%BACKUP_DIR%\nfpm.yaml" >nul
if errorlevel 1 goto :backup_failed

powershell -NoProfile -Command ^
    "$path = $env:CONFIG_PATH;" ^
    "$version = $env:LUNABOX_ASSET_VERSION;" ^
    "$lines = [System.IO.File]::ReadAllLines($path);" ^
    "$inInfo = $false; $updated = 0;" ^
    "for ($index = 0; $index -lt $lines.Length; $index++) {" ^
    "  $line = $lines[$index];" ^
    "  if ($line -match '^info:\s*$') { $inInfo = $true; continue };" ^
    "  if ($inInfo -and $line -match '^\S') { $inInfo = $false };" ^
    "  if ($inInfo -and $line -match '^(\s+)version:\s*.*$') {" ^
    "    $lines[$index] = $Matches[1] + 'version: ' + $version; $updated++" ^
    "  }" ^
    "};" ^
    "if ($updated -ne 1) { throw 'Unable to find exactly one info.version field' };" ^
    "[System.IO.File]::WriteAllLines($path, $lines, [System.Text.UTF8Encoding]::new($false))"
if errorlevel 1 goto :update_failed

echo Updating Wails build assets for LunaBox %VERSION%...
wails3 update build-assets -name LunaBox -binaryname LunaBox -config "%CONFIG_PATH%" -dir build
if errorlevel 1 goto :update_failed

call :restore_linux_assets
if errorlevel 1 goto :restore_failed

rmdir /s /q "%BACKUP_DIR%"
echo Updated Wails build assets to version %VERSION%.
echo Preserved %LINUX_DESKTOP_PATH% and %LINUX_NFPM_PATH%.
exit /b 0

:restore_linux_assets
copy /y "%BACKUP_DIR%\desktop" "%LINUX_DESKTOP_PATH%" >nul
if errorlevel 1 exit /b 1
copy /y "%BACKUP_DIR%\nfpm.yaml" "%LINUX_NFPM_PATH%" >nul
if errorlevel 1 exit /b 1
exit /b 0

:update_failed
set "SCRIPT_RESULT=1"
call :restore_linux_assets
if errorlevel 1 goto :restore_failed
copy /y "%BACKUP_DIR%\config.yml" "%CONFIG_PATH%" >nul
rmdir /s /q "%BACKUP_DIR%"
echo ERROR: Wails build assets were not updated.
exit /b %SCRIPT_RESULT%

:restore_failed
echo ERROR: Unable to restore the custom Linux build assets.
echo Backup directory: %BACKUP_DIR%
exit /b 1

:backup_failed
rmdir /s /q "%BACKUP_DIR%"
echo ERROR: Unable to back up the build asset files.
exit /b 1

:usage
echo Usage: scripts\update-build-assets.bat ^<version^>
echo Example: scripts\update-build-assets.bat 1.12.0
exit /b 1
