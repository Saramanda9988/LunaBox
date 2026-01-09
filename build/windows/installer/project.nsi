Unicode true

####
## Please note: Template replacements don't work in this file. They are provided with default defines like
## mentioned underneath.
## If the keyword is not defined, "wails_tools.nsh" will populate them with the values from ProjectInfo.
## If they are defined here, "wails_tools.nsh" will not touch them. This allows to use this project.nsi manually
## from outside of Wails for debugging and development of the installer.
##
## For development first make a wails nsis build to populate the "wails_tools.nsh":
## > wails build --target windows/amd64 --nsis
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

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
# !define MUI_WELCOMEFINISHPAGE_BITMAP "resources\leftimage.bmp" #Include this to add a bitmap on the left side of the Welcome Page. Must be a size of 164x314
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!define MUI_PAGE_CUSTOMFUNCTION_PRE skip_directory_page
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
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
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}" # Default installing folder ($PROGRAMFILES is Program Files folder).
ShowInstDetails show # This will always show the installation details.

# Variable to store whether we're doing an update
Var IS_UPDATE
# Variable to store whether to delete user data during uninstall
Var UN_DELETE_USERDATA

Function .onInit
   !insertmacro wails.checkArchitecture

   # Initialize update flag
   StrCpy $IS_UPDATE "0"

   # Check if old version is installed FIRST (before process check)
   SetRegView 64
   ReadRegStr $0 HKLM "${UNINST_KEY}" "DisplayVersion"
   ReadRegStr $1 HKLM "${UNINST_KEY}" "UninstallString"
   ReadRegStr $2 HKLM "${UNINST_KEY}" "InstallLocation"

   ${If} $0 != ""
   ${AndIf} $1 != ""
      # Old version found - ask user what to do
      StrCpy $IS_UPDATE "1"

      ${If} $2 == ""
         # InstallLocation is empty, extract from UninstallString
         # UninstallString format: "C:\Path\To\uninstall.exe"
         # Remove quotes from start and end
         StrCpy $3 $1 "" 1  ; Remove first quote
         StrCpy $3 $3 -1   ; Remove last quote
         # Get directory path from uninstaller path
         ${GetParent} $3 $2
      ${EndIf}

      # Store the old install path to $INSTDIR so we install to the same location
      StrCpy $INSTDIR $2

      # In silent mode, always update
      IfSilent run_silent_uninstall

      # Ask user: Update or Cancel
      MessageBox MB_YESNO|MB_ICONQUESTION "检测到 LunaBox 已安装版本 $0$\n$\n要更新到版本 ${INFO_PRODUCTVERSION} 吗$\n$\n是-自动更新保留数据$\n否-退出安装程序" IDYES run_uninstall IDNO cancel_update

      run_silent_uninstall:
      run_uninstall:
         # Check if process is running before uninstall
         FindWindow $5 "" "LunaBox"
         ${If} $5 != 0
            # Terminate the process silently
            nsExec::ExecToStack 'taskkill /F /IM "${PRODUCT_EXECUTABLE}"'
            Sleep 2000
         ${EndIf}

         # $3 already contains the uninstaller path without quotes (from lines 110-111)
         # Execute uninstaller silently from the old install location
         ExecWait '"$3" /S _?=$2' $4

         # Check if uninstall succeeded (non-zero return code indicates error)
         ${If} $4 != "0"
            # Uninstall failed, log the error but continue
            DetailPrint "Warning: Uninstaller returned error code $4"
         ${EndIf}

         # Uninstaller ran, skip process check since we already handled it
         Goto init_done

      cancel_update:
         Quit
   ${EndIf}

   # Check if LunaBox is running
   check_process:
   FindWindow $5 "" "LunaBox"
   ${If} $5 != 0
      IfSilent silent_kill ask_kill

      ask_kill:
         MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION '检测到 LunaBox 正在运行。$\n$\n请关闭 LunaBox 后点击"重试"继续安装，或点击"取消"退出安装程序。' IDRETRY check_process IDCANCEL cancel_install

      silent_kill:
         # Silent mode: automatically terminate the process
         nsExec::ExecToStack 'taskkill /F /IM "${PRODUCT_EXECUTABLE}"'
         Sleep 2000
         Goto init_done

      cancel_install:
         Quit
   ${EndIf}

   init_done:
FunctionEnd

# Skip directory page when updating (already set $INSTDIR to old location)
Function skip_directory_page
   ${If} $IS_UPDATE == "1"
      Abort
   ${EndIf}
FunctionEnd

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

    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR

    !insertmacro wails.files

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

