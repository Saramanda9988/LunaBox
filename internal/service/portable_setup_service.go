package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"lunabox/internal/protocol"
	"lunabox/internal/utils/apputils"
)

// PortableSetupService exposes setup helpers needed only by portable builds.
// Packaged builds receive their custom URL protocol association from Wails.
type PortableSetupService struct {
	ctx context.Context
}

func NewPortableSetupService() *PortableSetupService {
	return &PortableSetupService{}
}

//wails:ignore
func (s *PortableSetupService) Init(ctx context.Context) {
	s.ctx = ctx
}

// PortableProtocolStatus describes the current lunabox:// scheme binding.
type PortableProtocolStatus struct {
	Registered     bool   `json:"registered"`
	RegisteredPath string `json:"registeredPath"`
	CurrentPath    string `json:"currentPath"`
	UpToDate       bool   `json:"upToDate"`
}

// PortableCLIStatus describes the lunacli.exe presence and PATH registration.
type PortableCLIStatus struct {
	Available   bool   `json:"available"`
	CLIPath     string `json:"cliPath"`
	CLIDir      string `json:"cliDir"`
	InstallPath string `json:"installPath"`
	InstallDir  string `json:"installDir"`
	Registered  bool   `json:"registered"`
}

// PortableSetupStatus is the aggregate snapshot consumed by the settings UI.
type PortableSetupStatus struct {
	BuildMode      string                 `json:"buildMode"`
	IsPortable     bool                   `json:"isPortable"`
	Platform       string                 `json:"platform"`
	ExecutablePath string                 `json:"executablePath"`
	Protocol       PortableProtocolStatus `json:"protocol"`
	CLI            PortableCLIStatus      `json:"cli"`
}

// GetStatus returns the portable protocol and lunacli registration state.
func (s *PortableSetupService) GetStatus() (PortableSetupStatus, error) {
	status := PortableSetupStatus{
		BuildMode:  apputils.GetBuildMode(),
		IsPortable: apputils.IsPortableMode(),
		Platform:   runtime.GOOS,
	}

	exe, err := os.Executable()
	if err == nil {
		if abs, absErr := filepath.Abs(exe); absErr == nil {
			status.ExecutablePath = abs
		} else {
			status.ExecutablePath = exe
		}
	}

	status.Protocol.CurrentPath = status.ExecutablePath
	if runtime.GOOS == "darwin" {
		status.Protocol.Registered = true
		status.Protocol.RegisteredPath = "LaunchServices / Info.plist"
		status.Protocol.UpToDate = true
	} else {
		registeredExe, err := protocol.GetRegisteredURLSchemeExe()
		if err != nil {
			return status, fmt.Errorf("query portable protocol status: %w", err)
		}
		status.Protocol.RegisteredPath = registeredExe
		status.Protocol.Registered = registeredExe != ""
		status.Protocol.UpToDate = status.Protocol.Registered &&
			strings.EqualFold(filepath.Clean(registeredExe), filepath.Clean(status.ExecutablePath))
	}

	cliExists, cliPath, cliErr := apputils.CLIExists()
	if cliErr != nil {
		return status, fmt.Errorf("probe lunacli: %w", cliErr)
	}
	status.CLI.Available = cliExists
	status.CLI.CLIPath = cliPath
	if cliPath != "" {
		status.CLI.CLIDir = filepath.Dir(cliPath)
	}
	installPath, err := apputils.GetCLIInstallPath()
	if err != nil {
		return status, fmt.Errorf("resolve lunacli install path: %w", err)
	}
	status.CLI.InstallPath = installPath
	if installPath != "" {
		status.CLI.InstallDir = filepath.Dir(installPath)
	}
	registered, err := apputils.IsCLIInstalled()
	if err != nil {
		return status, fmt.Errorf("query CLI install status: %w", err)
	}
	status.CLI.Registered = registered

	return status, nil
}

// RegisterProtocol writes the lunabox:// association required by a Windows
// portable build. Installed builds are managed by Wails during packaging.
func (s *PortableSetupService) RegisterProtocol() (PortableSetupStatus, error) {
	if !apputils.IsPortableMode() {
		return PortableSetupStatus{}, fmt.Errorf("安装版协议由 Wails 安装程序管理")
	}
	if runtime.GOOS == "darwin" {
		return s.GetStatus()
	}
	if err := protocol.RegisterPortableURLScheme(""); err != nil {
		return PortableSetupStatus{}, fmt.Errorf("register portable protocol: %w", err)
	}
	return s.GetStatus()
}

// UnregisterProtocol removes the current-user association created for a
// Windows portable build.
func (s *PortableSetupService) UnregisterProtocol() (PortableSetupStatus, error) {
	if !apputils.IsPortableMode() {
		return PortableSetupStatus{}, fmt.Errorf("安装版协议由 Wails 安装程序管理")
	}
	if runtime.GOOS == "darwin" {
		return s.GetStatus()
	}
	if err := protocol.UnregisterPortableURLScheme(); err != nil {
		return PortableSetupStatus{}, fmt.Errorf("unregister portable protocol: %w", err)
	}
	return s.GetStatus()
}

// RegisterCLIPath adds the lunacli.exe directory to the per-user PATH.
func (s *PortableSetupService) RegisterCLIPath() (PortableSetupStatus, error) {
	if _, err := apputils.InstallCLI(); err != nil {
		return PortableSetupStatus{}, fmt.Errorf("install lunacli: %w", err)
	}
	return s.GetStatus()
}

// UnregisterCLIPath removes the lunacli registration for the current platform.
func (s *PortableSetupService) UnregisterCLIPath() (PortableSetupStatus, error) {
	if _, err := apputils.UninstallCLI(); err != nil {
		return PortableSetupStatus{}, fmt.Errorf("uninstall lunacli: %w", err)
	}
	return s.GetStatus()
}
