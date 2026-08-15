//go:build darwin

package importer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func findSteamInstallPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("无法定位用户目录: %w", err)
	}
	return findSteamInstallPathInHome(homeDir)
}

func findSteamInstallPathInHome(homeDir string) (string, error) {
	steamPath := filepath.Join(homeDir, "Library", "Application Support", "Steam")
	info, err := os.Stat(filepath.Join(steamPath, "steamapps"))
	if err != nil || !info.IsDir() {
		if err != nil {
			return "", fmt.Errorf("未找到有效的 Steam 本地库: %w", err)
		}
		return "", fmt.Errorf("未找到有效的 Steam 本地库")
	}
	return steamPath, nil
}

func shouldUpdateLocalSteamLaunchFields(conflict existingGameConflict) bool {
	return conflict.Type == ConflictTypeSource && strings.TrimSpace(conflict.Game.Path) == ""
}
