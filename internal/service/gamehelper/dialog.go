package gamehelper

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"lunabox/internal/wailsruntime"
)

// ExecutableDialogDirectory derives the initial directory for an executable
// picker. Wails v3 open dialogs do not support preselecting a filename.
func ExecutableDialogDirectory(currentPath string) string {
	currentPath = strings.TrimSpace(currentPath)
	if currentPath == "" {
		return ""
	}

	cleanPath := filepath.Clean(currentPath)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		absPath = cleanPath
	}

	info, err := os.Stat(absPath)
	if err == nil {
		if info.IsDir() {
			if IsMacAppBundlePath(absPath) {
				return filepath.Dir(absPath)
			}
			return absPath
		}
		return filepath.Dir(absPath)
	}

	if filepath.Ext(absPath) == "" {
		return ""
	}

	parentDir := filepath.Dir(absPath)
	if parentInfo, statErr := os.Stat(parentDir); statErr == nil && parentInfo.IsDir() {
		return parentDir
	}

	return ""
}

// IsMacAppBundlePath reports whether path points at a macOS .app bundle.
func IsMacAppBundlePath(path string) bool {
	return goruntime.GOOS == "darwin" && strings.EqualFold(filepath.Ext(strings.TrimSpace(path)), ".app")
}

// ExecutableOpenDialogOptions builds open-dialog options for selecting a game executable.
// On macOS the filters are omitted so Unix executables with no extension stay selectable
// and .app bundles can be picked as package files.
func ExecutableOpenDialogOptions(title, defaultDirectory string) wailsruntime.OpenDialogOptions {
	options := wailsruntime.OpenDialogOptions{
		Title:     title,
		Directory: defaultDirectory,
	}
	if goruntime.GOOS == "darwin" {
		options.ResolvesAliases = true
		options.TreatsFilePackagesAsDirectories = false
		return options
	}

	options.Filters = []wailsruntime.FileFilter{
		executableFileFilter(),
		allFilesFileFilter(),
	}
	return options
}

// WineRunnerOpenDialogOptions mirrors the executable selector but lets the user browse
// into macOS .app packages so they can target a binary inside the bundle.
func WineRunnerOpenDialogOptions(title, defaultDirectory string) wailsruntime.OpenDialogOptions {
	options := ExecutableOpenDialogOptions(title, defaultDirectory)
	if goruntime.GOOS == "darwin" {
		options.TreatsFilePackagesAsDirectories = true
	}
	return options
}

func executableFileFilter() wailsruntime.FileFilter {
	switch goruntime.GOOS {
	case "darwin":
		return wailsruntime.FileFilter{
			DisplayName: "Applications and Executables",
			Pattern:     "*.app;*.exe;*.bat;*.cmd",
		}
	default:
		return wailsruntime.FileFilter{
			DisplayName: "Executables",
			Pattern:     "*.exe;*.bat;*.cmd;*.lnk",
		}
	}
}

func allFilesFileFilter() wailsruntime.FileFilter {
	return wailsruntime.FileFilter{
		DisplayName: "All Files",
		Pattern:     "*.*",
	}
}
