Unicode true

####
## Please note: Template replacements don't work in this file. They are provided with default defines like
## mentioned underneath.
## If the keyword is not defined, "wails_tools.nsh" will populate them with the values from ProjectInfo.
## If they are defined here, "wails_tools.nsh" will not touch them. This allows to use this project.nsi manually
## from outside of Wails for debugging and development of the installer.
##
## Refresh generated metadata and helper macros with:
## > wails3 update build-assets -name LunaBox -binaryname LunaBox -config build/config.yml -dir build
## Then you can call makensis on this file with specifying the path to your binary:
## For a AMD64 only installer:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app.exe
## For a ARM64 only installer:
## > makensis -DARG_WAILS_ARM64_BINARY=..\..\bin\app.exe
## For a installer with both architectures:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app-amd64.exe -DARG_WAILS_ARM64_BINARY=..\..\bin\app-arm64.exe
####
## The following information is taken from the ProjectInfo file, but they can be overwritten here.
####
## !define INFO_PROJECTNAME    "MyProject" # Default "{{.Name}}"
## !define INFO_COMPANYNAME    "MyCompany" # Default "{{.Info.CompanyName}}"
## !define INFO_PRODUCTNAME    "MyProduct" # Default "{{.Info.ProductName}}"
## !define INFO_PRODUCTVERSION "1.0.0"     # Default "{{.Info.ProductVersion}}"
## !define INFO_COPYRIGHT      "Copyright" # Default "{{.Info.Copyright}}"
###
## !define PRODUCT_EXECUTABLE  "Application.exe"      # Default "${INFO_PROJECTNAME}.exe"
## !define UNINST_KEY_NAME     "UninstKeyInRegistry"  # Default "${INFO_COMPANYNAME}${INFO_PRODUCTNAME}"
####
## !define REQUEST_EXECUTION_LEVEL "admin"            # Default "admin"  see also https://nsis.sourceforge.io/Docs/Chapter4.html
####
## apps/desktop/tools/release 用这个脚本打安装包，用下面的 define 指定输入与输出（脚本沿用 Wails 版，
## 安装位置与卸载项不变，所以会直接替换已安装的 Wails 版）：
##   LUNABOX_PAYLOAD_DIR  lunacli.exe、7z、duckdb.dll（与 LunaBoxUpdater.exe）所在的目录，默认 ..\..\bin
##   LUNABOX_OUTFILE      安装包的路径，默认 ..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe
##   LUNABOX_NO_WEBVIEW2  不安装 WebView2 运行时
##   LUNABOX_NO_UPDATER   不安装 LunaBoxUpdater.exe
####
!ifndef LUNABOX_PAYLOAD_DIR
    !define LUNABOX_PAYLOAD_DIR "..\..\bin"
!endif
####
## Include the wails tools
####
!include "wails_tools.nsh"

# The version information for this two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true


!include "MUI.nsh"
!include "nsDialogs.nsh"
!include "WinMessages.nsh"
!include "LogicLib.nsh"
!include "StrFunc.nsh"

# Declare string functions for installer
${StrRep}

# Declare string functions for uninstaller (with Un prefix)
${UnStrRep}

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
# !define MUI_WELCOMEFINISHPAGE_BITMAP "resources\leftimage.bmp" #Include this to add a bitmap on the left side of the Welcome Page. Must be a size of 164x314
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!define MUI_PAGE_CUSTOMFUNCTION_PRE skip_directory_page
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
Page custom ShowCLIOptions ValidateCLIOptions # CLI 选项页面
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

# Uninstaller pages
!insertmacro MUI_UNPAGE_CONFIRM
UninstPage custom un.ShowUserDataOptions un.ValidateUserDataOptions
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "SimpChinese" # Set the Language of the installer

## The following two statements can be used to sign the installer and the uninstaller. The path to the binaries are provided in %1
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'

Name "${INFO_PRODUCTNAME}"
!ifdef LUNABOX_OUTFILE
OutFile "${LUNABOX_OUTFILE}"
!else
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
!endif
InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}" # Default installing folder ($PROGRAMFILES is Program Files folder).
ShowInstDetails show # This will always show the installation details.

# Variable to store whether we're doing an update
Var IS_UPDATE
# Variable to store whether to delete user data during uninstall
Var UN_DELETE_USERDATA
# Variable to store whether to install CLI and add to PATH
Var INSTALL_CLI_TO_PATH
# Variable to store the checkbox handle
Var CHECKBOX_CLI

Function .onInit
   !insertmacro wails.checkArchitecture

   # Only one installer at a time: two installers would write the same directory, registry key and PATH at once.
   # 183 = ERROR_ALREADY_EXISTS; the mutex is released when this installer exits.
   System::Call 'kernel32::CreateMutex(p 0, i 0, t "LunaBoxInstallerMutex") p .r0 ?e'
   Pop $0
   ${If} $0 == 183
      ${IfNot} ${Silent}
         MessageBox MB_OK|MB_ICONEXCLAMATION "LunaBox 安装程序已在运行，请等待它完成。"
      ${EndIf}
      Quit
   ${EndIf}

   # Initialize update flag
   StrCpy $IS_UPDATE "0"

   # Initialize CLI installation flag
   # Check registry to see if CLI was installed in previous version
   SetRegView 64
   ReadRegStr $0 HKLM "${UNINST_KEY}" "CLIInstalled"
   ${If} $0 == "1"
       # Was installed before (registry flag), keep it during update
       StrCpy $INSTALL_CLI_TO_PATH "1"
   ${ElseIf} $0 == "0"
       # Was explicitly NOT installed before, keep it that way
       StrCpy $INSTALL_CLI_TO_PATH "0"
   ${Else}
       # Registry flag not found (maybe older version or first install)
       # Check if lunacli.exe exists in the target directory
       # Need to find install location first
       ReadRegStr $1 HKLM "${UNINST_KEY}" "InstallLocation"
       ${If} $1 != ""
           IfFileExists "$1\lunacli.exe" 0 +3
               StrCpy $INSTALL_CLI_TO_PATH "1"
               Goto cli_check_done
       ${EndIf}
       
       # Default to checked for fresh install
       StrCpy $INSTALL_CLI_TO_PATH "1"
       cli_check_done:
   ${EndIf}

   # An installed version is updated in place, the way Tauri's installer does it: the install section
   # overwrites its files, uninstaller and registry entries. Its uninstaller is not run, so cancelling or a
   # failed install leaves it in place, and its PATH entry and shortcuts are kept as they are.
   SetRegView 64
   ReadRegStr $0 HKLM "${UNINST_KEY}" "DisplayVersion"
   ReadRegStr $1 HKLM "${UNINST_KEY}" "UninstallString"
   ReadRegStr $2 HKLM "${UNINST_KEY}" "InstallLocation"

   ${If} $0 != ""
   ${AndIf} $1 != ""
      StrCpy $IS_UPDATE "1"

      ${If} $2 == ""
         # InstallLocation is empty: take the directory of the uninstaller, from "C:\Path\To\uninstall.exe"
         StrCpy $3 $1 "" 1  ; Remove first quote
         StrCpy $3 $3 -1   ; Remove last quote
         ${GetParent} $3 $2
      ${EndIf}

      # Install to the same location (the directory page is skipped)
      StrCpy $INSTDIR $2

      # Nothing has been changed yet, so "No" just quits. Silent mode always updates.
      ${IfNot} ${Silent}
         MessageBox MB_YESNO|MB_ICONQUESTION "检测到 LunaBox 已安装版本 $0$\n$\n要更新到版本 ${INFO_PRODUCTVERSION} 吗？$\n$\n是 - 覆盖安装，保留数据与设置$\n否 - 退出安装程序" IDYES +2
         Quit
      ${EndIf}
   ${EndIf}
FunctionEnd

# Like Tauri's CheckIfAppIsRunning: before files are written or removed, ask to end LunaBox if it is running
# (silent mode ends it without asking), and abort if it cannot be ended. Files of a running exe can't be replaced.
# The process is looked up by image name: tasklist /FO CSV prints a line starting with a quote for each match,
# and a localized "INFO: ..." line when there is none.
!macro CheckIfAppIsRunningFunction UN
Function ${UN}CheckIfAppIsRunning
    nsExec::ExecToStack '"$SYSDIR\tasklist.exe" /FI "IMAGENAME eq ${PRODUCT_EXECUTABLE}" /FO CSV /NH'
    Pop $0
    Pop $1
    StrCpy $1 $1 1
    ${If} $0 != "0"
    ${OrIf} $1 != '"'
        Return
    ${EndIf}

    ${IfNot} ${Silent}
        !if "${UN}" == "un."
            MessageBox MB_OKCANCEL|MB_ICONEXCLAMATION "LunaBox 正在运行。$\n$\n点击“确定”结束 LunaBox 并继续卸载，点击“取消”退出。" IDOK +2
            Abort "LunaBox 正在运行，卸载已取消。"
        !else
            MessageBox MB_OKCANCEL|MB_ICONEXCLAMATION "LunaBox 正在运行。$\n$\n点击“确定”结束 LunaBox 并继续安装，点击“取消”退出。" IDOK +2
            Abort "LunaBox 正在运行，安装已取消。"
        !endif
    ${EndIf}

    # No /T: games started by LunaBox are its child processes and must keep running
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "${PRODUCT_EXECUTABLE}"'
    Pop $0
    Pop $1
    Sleep 1000

    nsExec::ExecToStack '"$SYSDIR\tasklist.exe" /FI "IMAGENAME eq ${PRODUCT_EXECUTABLE}" /FO CSV /NH'
    Pop $0
    Pop $1
    StrCpy $1 $1 1
    ${If} $0 == "0"
    ${AndIf} $1 == '"'
        Abort "无法结束 LunaBox，请手动关闭后重试。"
    ${EndIf}
FunctionEnd
!macroend
!insertmacro CheckIfAppIsRunningFunction ""
!insertmacro CheckIfAppIsRunningFunction "un."

# Skip directory page when updating (already set $INSTDIR to old location)
Function skip_directory_page
   ${If} $IS_UPDATE == "1"
      Abort
   ${EndIf}
FunctionEnd

# Show CLI installation options page
Function ShowCLIOptions
   # Skip in silent mode
   IfSilent 0 +2
      Abort

   !insertmacro MUI_HEADER_TEXT "命令行工具" "选择是否安装命令行支持"

   nsDialogs::Create 1018
   Pop $0

   ${NSD_CreateLabel} 0 0 100% 50u "LunaBox 提供命令行工具 (CLI) 支持，您可以在终端中使用 'lunacli' 命令来启动和管理游戏。"
   Pop $0

   ${NSD_CreateCheckbox} 15 65u 100% 15u "安装命令行工具 (CLI) 并添加到 PATH (推荐)"
   Pop $CHECKBOX_CLI
   
   # Set initial state based on $INSTALL_CLI_TO_PATH
   ${If} $INSTALL_CLI_TO_PATH == "1"
      ${NSD_SetState} $CHECKBOX_CLI ${BST_CHECKED}
   ${Else}
      ${NSD_SetState} $CHECKBOX_CLI ${BST_UNCHECKED}
   ${EndIf}

   ${NSD_CreateLabel} 30 85u 100% 20u "注意：添加到 PATH 后，您可以在任何位置的终端中直接使用 'lunacli' 命令。"
   Pop $0

   nsDialogs::Show
FunctionEnd

# Validate CLI options (save the checkbox state)
Function ValidateCLIOptions
   ${NSD_GetState} $CHECKBOX_CLI $0
   ${If} $0 == ${BST_CHECKED}
      StrCpy $INSTALL_CLI_TO_PATH "1"
   ${Else}
      StrCpy $INSTALL_CLI_TO_PATH "0"
   ${EndIf}
FunctionEnd

# Writes the PowerShell line, to the script open as $8, that stops the script before it writes PATH back
# when the machine PATH was not read as a non-empty REG_SZ / REG_EXPAND_SZ: GetValue returns '' for a
# missing value, and an emptied PATH must not be written back with only the install directory in it.
# GetValueKind throws for a missing value, which also stops the script ($ErrorActionPreference = 'Stop').
!macro PathReadCheck
    FileWrite $8 "  $$kind = $$key.GetValueKind('Path')$\r$\n"
    FileWrite $8 "  if (($$kind -ne [Microsoft.Win32.RegistryValueKind]::ExpandString -and $$kind -ne [Microsoft.Win32.RegistryValueKind]::String) -or [string]::IsNullOrWhiteSpace($$path)) {$\r$\n"
    FileWrite $8 "    throw ('Refusing to modify the machine PATH: it was not read as a non-empty string (' + $$kind + ')')$\r$\n"
    FileWrite $8 "  }$\r$\n"
!macroend

# Add $INSTDIR to the machine PATH without round-tripping PATH through NSIS variables.
# NSIS strings are commonly limited to 1024 chars; long PATH values would otherwise be truncated.
Function AddInstallDirToPath
    SetRegView 64
    InitPluginsDir

    ${StrRep} $7 "$INSTDIR" "'" "''"
    StrCpy $9 "$PLUGINSDIR\lunabox-add-path.ps1"
    FileOpen $8 $9 w
    FileWrite $8 "$$ErrorActionPreference = 'Stop'$\r$\n"
    FileWrite $8 "$$installDir = '$7'$\r$\n"
    FileWrite $8 "$$keyPath = 'SYSTEM\CurrentControlSet\Control\Session Manager\Environment'$\r$\n"
    FileWrite $8 "$$key = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey($$keyPath, $$true)$\r$\n"
    FileWrite $8 "if ($$null -eq $$key) { throw 'Environment registry key not found' }$\r$\n"
    FileWrite $8 "try {$\r$\n"
    FileWrite $8 "  $$path = [string]$$key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)$\r$\n"
    FileWrite $8 "  $$backupDir = Join-Path ([Environment]::GetFolderPath('CommonApplicationData')) 'LunaBox\environment-backups'$\r$\n"
    FileWrite $8 "  [void][System.IO.Directory]::CreateDirectory($$backupDir)$\r$\n"
    FileWrite $8 "  $$backupFile = Join-Path $$backupDir ('machine-path-before-install-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '.txt')$\r$\n"
    FileWrite $8 "  [System.IO.File]::WriteAllText($$backupFile, $$path, [System.Text.Encoding]::UTF8)$\r$\n"
    FileWrite $8 "  Write-Output ('Backed up machine PATH to ' + $$backupFile)$\r$\n"
    # Never write PATH back from a value that was not read as a non-empty string (see PathReadCheck)
    !insertmacro PathReadCheck
    FileWrite $8 "  $$entries = @($$path -split ';' | Where-Object { $$_ -ne '' })$\r$\n"
    FileWrite $8 "  $$normalizedInstallDir = $$installDir.Trim().Trim([char]34).TrimEnd([char]92)$\r$\n"
    FileWrite $8 "  $$exists = $$false$\r$\n"
    FileWrite $8 "  foreach ($$entry in $$entries) {$\r$\n"
    FileWrite $8 "    if ($$entry.Trim().Trim([char]34).TrimEnd([char]92) -ieq $$normalizedInstallDir) { $$exists = $$true; break }$\r$\n"
    FileWrite $8 "  }$\r$\n"
    FileWrite $8 "  if (-not $$exists) {$\r$\n"
    FileWrite $8 "    $$entries += $$installDir$\r$\n"
    FileWrite $8 "    $$key.SetValue('Path', [string]::Join(';', $$entries), [Microsoft.Win32.RegistryValueKind]::ExpandString)$\r$\n"
    FileWrite $8 "  }$\r$\n"
    FileWrite $8 "} finally {$\r$\n"
    FileWrite $8 "  $$key.Close()$\r$\n"
    FileWrite $8 "}$\r$\n"
    FileClose $8

    nsExec::ExecToStack '"$SYSDIR\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -ExecutionPolicy Bypass -File "$9"'
    Pop $6
    Pop $5
    ${If} $6 != "0"
        DetailPrint "Warning: failed to add install directory to PATH: $5"
    ${Else}
        DetailPrint "$5"
    ${EndIf}
    Delete $9
FunctionEnd

# Remove $INSTDIR from the machine PATH without round-tripping PATH through NSIS variables.
# Defined for the uninstaller and for the installer, which removes the PATH entry of the version it
# overwrites when the user no longer selects the CLI.
!macro RemoveInstallDirFromPathFunction UN
Function ${UN}RemoveInstallDirFromPath
    SetRegView 64
    InitPluginsDir

    !if "${UN}" == "un."
        ${UnStrRep} $7 "$INSTDIR" "'" "''"
    !else
        ${StrRep} $7 "$INSTDIR" "'" "''"
    !endif
    StrCpy $9 "$PLUGINSDIR\lunabox-remove-path.ps1"
    FileOpen $8 $9 w
    FileWrite $8 "$$ErrorActionPreference = 'Stop'$\r$\n"
    FileWrite $8 "$$installDir = '$7'$\r$\n"
    FileWrite $8 "$$keyPath = 'SYSTEM\CurrentControlSet\Control\Session Manager\Environment'$\r$\n"
    FileWrite $8 "$$key = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey($$keyPath, $$true)$\r$\n"
    FileWrite $8 "if ($$null -eq $$key) { throw 'Environment registry key not found' }$\r$\n"
    FileWrite $8 "try {$\r$\n"
    FileWrite $8 "  $$path = [string]$$key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)$\r$\n"
    FileWrite $8 "  $$backupDir = Join-Path ([Environment]::GetFolderPath('CommonApplicationData')) 'LunaBox\environment-backups'$\r$\n"
    FileWrite $8 "  [void][System.IO.Directory]::CreateDirectory($$backupDir)$\r$\n"
    FileWrite $8 "  $$backupFile = Join-Path $$backupDir ('machine-path-before-uninstall-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '.txt')$\r$\n"
    FileWrite $8 "  [System.IO.File]::WriteAllText($$backupFile, $$path, [System.Text.Encoding]::UTF8)$\r$\n"
    FileWrite $8 "  Write-Output ('Backed up machine PATH to ' + $$backupFile)$\r$\n"
    # Never write PATH back from a value that was not read as a non-empty string (see PathReadCheck)
    !insertmacro PathReadCheck
    FileWrite $8 "  $$normalizedInstallDir = $$installDir.Trim().Trim([char]34).TrimEnd([char]92)$\r$\n"
    FileWrite $8 "  $$newEntries = New-Object System.Collections.Generic.List[string]$\r$\n"
    FileWrite $8 "  $$changed = $$false$\r$\n"
    FileWrite $8 "  foreach ($$entry in @($$path -split ';')) {$\r$\n"
    FileWrite $8 "    if ($$entry.Trim().Trim([char]34).TrimEnd([char]92) -ieq $$normalizedInstallDir) { $$changed = $$true; continue }$\r$\n"
    FileWrite $8 "    [void]$$newEntries.Add($$entry)$\r$\n"
    FileWrite $8 "  }$\r$\n"
    FileWrite $8 "  if ($$changed) {$\r$\n"
    FileWrite $8 "    $$key.SetValue('Path', [string]::Join(';', $$newEntries.ToArray()).Trim(';'), [Microsoft.Win32.RegistryValueKind]::ExpandString)$\r$\n"
    FileWrite $8 "  }$\r$\n"
    FileWrite $8 "} finally {$\r$\n"
    FileWrite $8 "  $$key.Close()$\r$\n"
    FileWrite $8 "}$\r$\n"
    FileClose $8

    nsExec::ExecToStack '"$SYSDIR\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -ExecutionPolicy Bypass -File "$9"'
    Pop $6
    Pop $5
    ${If} $6 != "0"
        DetailPrint "Warning: failed to remove install directory from PATH: $5"
    ${Else}
        DetailPrint "$5"
    ${EndIf}
    Delete $9
FunctionEnd
!macroend
!insertmacro RemoveInstallDirFromPathFunction ""
!insertmacro RemoveInstallDirFromPathFunction "un."

# Uninstaller: Initialize the user data options page
Function un.ShowUserDataOptions
   # Skip this page in silent mode (preserve user data)
   IfSilent 0 +2
      Abort

   !insertmacro MUI_HEADER_TEXT "卸载选项" "请选择是否删除用户数据"

   nsDialogs::Create 1018
   Pop $0

   ${NSD_CreateLabel} 0 0 100% 40u "是否要同时删除 LunaBox 的用户数据？$\n$\n用户数据包括:$\n  - 配置文件$\n  - 游戏数据库$\n  - 备份文件$\n$\n数据位置: $APPDATA\LunaBox 和 $LOCALAPPDATA\LunaBox"
   Pop $0

   ${NSD_CreateRadioButton} 15 60u 100% 15u "保留用户数据 (推荐)"
   Pop $1
   ${NSD_SetState} $1 ${BST_CHECKED}
   ${NSD_OnClick} $1 un.RadioButtonClicked

   ${NSD_CreateRadioButton} 15 85u 100% 15u "删除所有用户数据"
   Pop $2
   ${NSD_OnClick} $2 un.RadioButtonClicked

   # Initialize to "keep data" by default
   StrCpy $UN_DELETE_USERDATA "0"

   nsDialogs::Show
FunctionEnd

# Handle radio button clicks
Function un.RadioButtonClicked
   Pop $0  # Get the control handle
   ${If} $0 == $1
      StrCpy $UN_DELETE_USERDATA "0"
   ${Else}
      StrCpy $UN_DELETE_USERDATA "1"
   ${EndIf}
FunctionEnd

# Validate the page (always allow next)
Function un.ValidateUserDataOptions
FunctionEnd

Section
    !insertmacro wails.setShellContext

    Call CheckIfAppIsRunning

    !ifndef LUNABOX_NO_WEBVIEW2
        !insertmacro wails.webview2runtime
    !endif

    SetOutPath $INSTDIR

    !insertmacro wails.files

    !ifndef LUNABOX_NO_UPDATER
        # Standalone updater runs from a temporary copy, so it can also replace its installed copy.
        File "${LUNABOX_PAYLOAD_DIR}\LunaBoxUpdater.exe"
    !endif

    # ARM64 builds dynamically link DuckDB; include the runtime DLL when the payload provides it.
    !if /FileExists "${LUNABOX_PAYLOAD_DIR}\duckdb.dll"
        File "${LUNABOX_PAYLOAD_DIR}\duckdb.dll"
    !endif

    !if /FileExists "${LUNABOX_PAYLOAD_DIR}\7z\7z.exe"
        SetOutPath "$INSTDIR\7z"
        File "${LUNABOX_PAYLOAD_DIR}\7z\7z.exe"
        !if /FileExists "${LUNABOX_PAYLOAD_DIR}\7z\7z.dll"
            File "${LUNABOX_PAYLOAD_DIR}\7z\7z.dll"
        !endif
        SetOutPath $INSTDIR
    !endif

    # Install CLI version and add to PATH only if user selected it
    ${If} $INSTALL_CLI_TO_PATH == "1"
        # Install CLI version (lunacli.exe) for command-line usage
        File "${LUNABOX_PAYLOAD_DIR}\lunacli.exe"

        # Add install directory to system PATH (for CLI usage)
        Call AddInstallDirToPath
        # Broadcast WM_SETTINGCHANGE to notify of PATH change
        # Use SendMessageTimeout to avoid hanging if a top-level window is unresponsive
        # 0xffff = HWND_BROADCAST, 2 = SMTO_ABORTIFHUNG, 200 = Timeout in ms for EACH window:
        # some apps (e.g. WPS Office) keep many windows that never answer, and 5000 ms each added up to minutes.
        System::Call 'user32::SendMessageTimeout(p 0xffff, i ${WM_SETTINGCHANGE}, p 0, t "Environment", i 2, i 200, *i .r0)'

        # Record that CLI was installed (for future updates)
        SetRegView 64
        WriteRegStr HKLM "${UNINST_KEY}" "CLIInstalled" "1"
    ${Else}
        # The overwritten version may have installed the CLI: remove it, and its PATH entry unless that
        # version recorded it had no CLI (versions before the CLIInstalled flag leave it empty)
        Delete "$INSTDIR\lunacli.exe"
        ${If} $IS_UPDATE == "1"
            SetRegView 64
            ReadRegStr $0 HKLM "${UNINST_KEY}" "CLIInstalled"
            ${If} $0 != "0"
                Call RemoveInstallDirFromPath
                System::Call 'user32::SendMessageTimeout(p 0xffff, i ${WM_SETTINGCHANGE}, p 0, t "Environment", i 2, i 200, *i .r0)'
            ${EndIf}
        ${EndIf}

        # Record that CLI was NOT installed
        SetRegView 64
        WriteRegStr HKLM "${UNINST_KEY}" "CLIInstalled" "0"
    ${EndIf}

    # Always create/recreate start menu shortcut (important for updates and pinned shortcuts)
    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    
    # Create desktop shortcut only during fresh install or if it doesn't exist during update
    ${If} $IS_UPDATE == "0"
        CreateShortcut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    ${Else}
        # During update, only create desktop shortcut if it doesn't exist
        IfFileExists "$DESKTOP\${INFO_PRODUCTNAME}.lnk" +2 0
            CreateShortcut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    ${EndIf}

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    !insertmacro wails.writeUninstaller

    # Write InstallLocation to registry for update detection
    SetRegView 64
    WriteRegStr HKLM "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
SectionEnd

Section "uninstall"
    !insertmacro wails.setShellContext

    Call un.CheckIfAppIsRunning

    # Remove install directory from PATH only if it was added during installation
    SetRegView 64
    ReadRegStr $0 HKLM "${UNINST_KEY}" "CLIInstalled"
    ${If} $0 == "1"
        # CLI was installed, remove from PATH
        Call un.RemoveInstallDirFromPath
        # Broadcast WM_SETTINGCHANGE to notify of PATH change
        # Use SendMessageTimeout to avoid hanging if a top-level window is unresponsive
        # 0xffff = HWND_BROADCAST, 2 = SMTO_ABORTIFHUNG, 200 = Timeout in ms for EACH window (see the installer section)
        System::Call 'user32::SendMessageTimeout(p 0xffff, i ${WM_SETTINGCHANGE}, p 0, t "Environment", i 2, i 200, *i .r0)'
    ${EndIf}

    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # Remove the WebView2 DataPath

    # Delete user data based on user's choice from the custom page
    ${If} $UN_DELETE_USERDATA == "1"
        SetShellVarContext current
        RMDir /r "$APPDATA\LunaBox"
        RMDir /r "$LOCALAPPDATA\LunaBox"
        !insertmacro wails.setShellContext
    ${EndIf}

    RMDir /r $INSTDIR

    # Delete shortcuts - use SetErrors to ignore errors if they don't exist
    SetErrors
    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro wails.deleteUninstaller
SectionEnd
