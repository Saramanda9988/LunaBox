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

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
# !define MUI_WELCOMEFINISHPAGE_BITMAP "resources\leftimage.bmp" #Include this to add a bitmap on the left side of the Welcome Page. Must be a size of 164x314
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_INSTFILES # Uinstalling page

!insertmacro MUI_LANGUAGE "English" # Set the Language of the installer

## The following two statements can be used to sign the installer and the uninstaller. The path to the binaries are provided in %1
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}" # Default installing folder ($PROGRAMFILES is Program Files folder).
ShowInstDetails show # This will always show the installation details.

Function .onInit
   !insertmacro wails.checkArchitecture
   
   # Check if LunaBox is running
   check_process:
   FindWindow $0 "" "LunaBox"
   ${If} $0 != 0
      IfSilent silent_kill ask_kill
      
      ask_kill:
         MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION '检测到 LunaBox 正在运行。$\n$\n请关闭 LunaBox 后点击"重试"继续安装，或点击"取消"退出安装程序。' IDRETRY check_process IDCANCEL cancel_install
      
      silent_kill:
         # Silent mode: automatically terminate the process
         nsExec::ExecToStack 'taskkill /F /IM "${PRODUCT_EXECUTABLE}"'
         Sleep 2000
         Goto check_done
      
      cancel_install:
         Quit
   ${EndIf}
   
   check_done:
   
   # Check if old version is installed
   SetRegView 64
   ReadRegStr $0 HKLM "${UNINST_KEY}" "DisplayVersion"
   ReadRegStr $1 HKLM "${UNINST_KEY}" "UninstallString"
   
   ${If} $0 != ""
   ${AndIf} $1 != ""
      # Already installed
      IfSilent run_uninstall show_prompt
      
      show_prompt:
         MessageBox MB_YESNO|MB_ICONQUESTION '检测到 LunaBox 已安装 (版本 $0)。$\n$\n是否更新到版本 ${INFO_PRODUCTVERSION}？$\n$\n(旧版本将被卸载，但您的数据会被保留。)' IDYES run_uninstall IDNO skip_uninstall
         
      run_uninstall:
         # Remove quotes and execute uninstaller
         StrCpy $2 $1 "" 1
         StrCpy $2 $2 -1
         
         # Run uninstaller silently to avoid user interaction during update
         # The uninstaller is configured to keep user data in silent mode
         ExecWait '"$2" /S _?=$INSTDIR'
         
         Goto done_check
         
      skip_uninstall:
         MessageBox MB_OK '将继续安装，新版本文件将覆盖旧版本。'
      
      done_check:
   ${EndIf}
FunctionEnd

Section
    !insertmacro wails.setShellContext

    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR

    !insertmacro wails.files

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    !insertmacro wails.writeUninstaller
SectionEnd

Section "uninstall"
    !insertmacro wails.setShellContext

    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # Remove the WebView2 DataPath

    # Check if running in silent mode (e.g. during update)
    IfSilent skip_userdata_silent ask_userdata

    ask_userdata:
        # Switch to current user context to correctly locate user data
        SetShellVarContext current
        
        # Ask user whether to delete user data
        MessageBox MB_YESNO '是否删除 LunaBox 的用户数据（配置、数据库、备份等）？$\n$\n数据位置:$\n$APPDATA\LunaBox$\n$LOCALAPPDATA\LunaBox' IDNO skip_delete_data
        
        # Delete data if user clicked Yes
        RMDir /r "$APPDATA\LunaBox"
        RMDir /r "$LOCALAPPDATA\LunaBox"
        
    skip_delete_data:
        # Restore shell context to 'all' (for admin install) to clean up Program Files and Shortcuts
        !insertmacro wails.setShellContext
        Goto skip_userdata

    skip_userdata_silent:
        # In silent mode, we preserve user data by default (safe for updates)
        
    skip_userdata:

    RMDir /r $INSTDIR

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro wails.deleteUninstaller
SectionEnd

