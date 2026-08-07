//go:build linux

package protocol

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	linuxDesktopID       = "org.wails.lunabox"
	linuxDesktopFileName = linuxDesktopID + ".desktop"
)

var legacyLinuxDesktopFileNames = []string{
	"io.github.saramanda9988.lunabox.desktop",
	"lunabox.desktop",
}

func RegisterURLScheme(exePath string) error {
	if strings.TrimSpace(exePath) == "" {
		var err error
		exePath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("get executable path: %w", err)
		}
	}
	exePath, err := filepath.Abs(filepath.Clean(exePath))
	if err != nil {
		return fmt.Errorf("normalize executable path: %w", err)
	}

	desktopPath, err := userDesktopFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(desktopPath), 0755); err != nil {
		return fmt.Errorf("create applications directory: %w", err)
	}
	if err := os.WriteFile(desktopPath, []byte(protocolDesktopEntry(exePath)), 0644); err != nil {
		return fmt.Errorf("write desktop entry: %w", err)
	}
	removeLegacyDesktopFile(desktopPath)

	if xdgMime, err := exec.LookPath("xdg-mime"); err == nil {
		if err := exec.Command(xdgMime, "default", linuxDesktopFileName, "x-scheme-handler/"+Scheme).Run(); err != nil {
			return fmt.Errorf("register xdg mime handler: %w", err)
		}
	} else {
		return fmt.Errorf("xdg-mime not found; install xdg-utils to register %s://", Scheme)
	}
	refreshDesktopDatabase(filepath.Dir(desktopPath))
	return nil
}

func RegisterPortableURLScheme(exePath string) error {
	return RegisterURLScheme(exePath)
}

func UnregisterURLScheme() error {
	applicationsDir, err := userApplicationsDir()
	if err != nil {
		return err
	}
	for _, desktopFileName := range append([]string{linuxDesktopFileName}, legacyLinuxDesktopFileNames...) {
		path := filepath.Join(applicationsDir, desktopFileName)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove desktop entry: %w", err)
		}
	}
	refreshDesktopDatabase(applicationsDir)
	return nil
}

func UnregisterPortableURLScheme() error {
	return UnregisterURLScheme()
}

func GetRegisteredURLSchemeExe() (string, error) {
	xdgMime, err := exec.LookPath("xdg-mime")
	if err != nil {
		return "", nil
	}
	out, err := exec.Command(xdgMime, "query", "default", "x-scheme-handler/"+Scheme).Output()
	if err != nil {
		return "", nil
	}
	desktopName := strings.TrimSpace(string(out))
	if desktopName == "" {
		return "", nil
	}

	for _, candidate := range desktopFileCandidates(desktopName) {
		exe, err := execFromDesktopFile(candidate)
		if err == nil && strings.TrimSpace(exe) != "" {
			return exe, nil
		}
	}
	return "", nil
}

func userDesktopFilePath() (string, error) {
	applicationsDir, err := userApplicationsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(applicationsDir, linuxDesktopFileName), nil
}

func userApplicationsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get user home: %w", err)
	}
	return filepath.Join(home, ".local", "share", "applications"), nil
}

func protocolDesktopEntry(exePath string) string {
	icon := protocolDesktopIcon(exePath)
	return strings.Join([]string{
		"[Desktop Entry]",
		"Version=1.0",
		"Type=Application",
		"Name=LunaBox",
		"Comment=LunaBox game library manager",
		"Exec=" + quoteDesktopExecArg(exePath) + " %u",
		"Terminal=false",
		"Icon=" + icon,
		"Categories=Utility;Game;",
		"StartupWMClass=" + linuxDesktopID,
		"MimeType=x-scheme-handler/" + Scheme + ";",
		"",
	}, "\n")
}

func protocolDesktopIcon(exePath string) string {
	portableIcon := filepath.Join(filepath.Dir(exePath), "appicon.png")
	if info, err := os.Stat(portableIcon); err == nil && !info.IsDir() {
		return portableIcon
	}
	return linuxDesktopID
}

func removeLegacyDesktopFile(currentDesktopPath string) {
	for _, desktopFileName := range legacyLinuxDesktopFileNames {
		legacyPath := filepath.Join(filepath.Dir(currentDesktopPath), desktopFileName)
		if legacyPath != currentDesktopPath {
			_ = os.Remove(legacyPath)
		}
	}
}

func desktopFileCandidates(name string) []string {
	candidates := make([]string, 0, 4)
	if filepath.IsAbs(name) {
		return append(candidates, name)
	}
	if userPath, err := userDesktopFilePath(); err == nil && filepath.Base(userPath) == name {
		candidates = append(candidates, userPath)
	}
	home, err := os.UserHomeDir()
	if err == nil {
		candidates = append(candidates, filepath.Join(home, ".local", "share", "applications", name))
	}
	candidates = append(candidates, filepath.Join("/usr/share/applications", name))
	return candidates
}

func execFromDesktopFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "Exec=") {
			continue
		}
		return extractDesktopExecPath(strings.TrimPrefix(line, "Exec=")), nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", nil
}

func extractDesktopExecPath(command string) string {
	command = strings.TrimSpace(stripDesktopFieldCodes(command))
	if command == "" {
		return ""
	}
	if strings.HasPrefix(command, `"`) {
		escaped := false
		var builder strings.Builder
		for _, r := range command[1:] {
			if escaped {
				builder.WriteRune(r)
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '"' {
				return builder.String()
			}
			builder.WriteRune(r)
		}
		return builder.String()
	}
	if idx := strings.Index(command, " "); idx >= 0 {
		return command[:idx]
	}
	return command
}

func stripDesktopFieldCodes(command string) string {
	for _, code := range []string{"%u", "%U", "%f", "%F"} {
		command = strings.ReplaceAll(command, code, "")
	}
	return command
}

func quoteDesktopExecArg(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", "$", "\\$")
	return `"` + replacer.Replace(value) + `"`
}

func refreshDesktopDatabase(dir string) {
	if updater, err := exec.LookPath("update-desktop-database"); err == nil {
		_ = exec.Command(updater, dir).Run()
	}
}
