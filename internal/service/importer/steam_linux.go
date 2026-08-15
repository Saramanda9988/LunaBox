//go:build linux

package importer

import (
	"fmt"
	"lunabox/internal/common/enums"
	"os"
	"path/filepath"
	"strings"
)

func findSteamInstallPath() (string, error) {
	for _, candidate := range steamLinuxInstallPathCandidates() {
		if path, ok := normalizeSteamLinuxInstallPath(candidate); ok {
			return path, nil
		}
	}
	return "", fmt.Errorf("未找到有效的 Steam 安装目录")
}

func shouldUpdateLocalSteamLaunchFields(conflict existingGameConflict) bool {
	return conflict.Type == ConflictTypeSource && strings.TrimSpace(conflict.Game.Path) == ""
}

func defaultSteamImportedLaunchMode() enums.LaunchMode {
	return enums.LaunchModeSteam
}

func steamLinuxInstallPathCandidates() []string {
	candidates := make([]string, 0, 12)
	addCandidate := func(path string) {
		path = strings.TrimSpace(path)
		if path != "" {
			candidates = append(candidates, path)
		}
	}

	for _, key := range []string{
		"STEAM_DIR",
		"STEAM_HOME",
		"STEAM_ROOT",
		"STEAM_COMPAT_CLIENT_INSTALL_PATH",
	} {
		addCandidate(os.Getenv(key))
	}

	if xdgDataHome := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdgDataHome != "" {
		addCandidate(filepath.Join(xdgDataHome, "Steam"))
	}

	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		addCandidate(filepath.Join(home, ".steam", "steam"))
		addCandidate(filepath.Join(home, ".steam", "root"))
		addCandidate(filepath.Join(home, ".steam", "debian-installation"))
		addCandidate(filepath.Join(home, ".local", "share", "Steam"))
		addCandidate(filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"))
		addCandidate(filepath.Join(home, "snap", "steam", "common", ".local", "share", "Steam"))
	}

	return uniqueSteamLinuxCandidates(candidates)
}

func uniqueSteamLinuxCandidates(paths []string) []string {
	result := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		key := filepath.Clean(expandHomePath(path))
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, path)
	}
	return result
}

func normalizeSteamLinuxInstallPath(path string) (string, bool) {
	path = expandHomePath(strings.TrimSpace(path))
	if path == "" {
		return "", false
	}

	cleaned, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", false
	}

	if isDir(filepath.Join(cleaned, "steamapps")) {
		return cleaned, true
	}
	if strings.EqualFold(filepath.Base(cleaned), "steamapps") && isDir(cleaned) {
		return filepath.Dir(cleaned), true
	}
	return "", false
}

func expandHomePath(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
