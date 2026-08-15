//go:build windows

package importer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func findSteamInstallPath() (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("未找到 Steam 安装信息: %w", err)
	}
	defer key.Close()

	for _, valueName := range []string{"SteamPath", "InstallPath"} {
		value, _, err := key.GetStringValue(valueName)
		if err != nil || strings.TrimSpace(value) == "" {
			continue
		}
		path, err := filepath.Abs(filepath.Clean(normalizeSteamPath(value)))
		if err != nil {
			continue
		}
		if info, err := os.Stat(filepath.Join(path, "steam.exe")); err == nil && !info.IsDir() {
			return path, nil
		}
		if info, err := os.Stat(filepath.Join(path, "steamapps")); err == nil && info.IsDir() {
			return path, nil
		}
	}

	return "", fmt.Errorf("未找到有效的 Steam 安装目录")
}

func shouldUpdateLocalSteamLaunchFields(existingGameConflict) bool {
	return false
}
