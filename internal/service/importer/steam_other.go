//go:build !windows && !darwin && !linux

package importer

import "fmt"

func findSteamInstallPath() (string, error) {
	return "", fmt.Errorf("Steam 本地库扫描当前仅支持 Windows/macOS/Linux")
}

func shouldUpdateLocalSteamLaunchFields(existingGameConflict) bool {
	return false
}
