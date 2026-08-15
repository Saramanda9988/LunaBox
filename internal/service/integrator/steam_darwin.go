//go:build darwin

package integrator

import (
	"context"
	"fmt"
	"lunabox/internal/models"
	"lunabox/internal/utils/processutils"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func resolveSteamPlatformTarget(_ context.Context, game models.Game) (SteamResult, error) {
	if _, err := findDarwinSteamExecutable(); err != nil {
		return SteamResult{
			Status: SteamLaunchStatus{State: SteamLaunchStateSteamNotInstalled},
		}, nil
	}

	steamRunning, _ := processutils.CheckIfProcessRunning("steam_osx")
	status := SteamLaunchStatus{
		SteamInstalled: true,
		SteamRunning:   steamRunning,
	}
	launchID, ok := nativeSteamLaunchID(game)
	if !ok {
		status.State = SteamLaunchStateNeedsImport
		return SteamResult{Status: status}, nil
	}

	status.State = SteamLaunchStateReady
	status.Ready = true
	status.LaunchID = launchID
	status.LaunchKind = "native"
	return SteamResult{Status: status}, nil
}

func importSteamPlatformShortcut(_ context.Context, _ models.Game) (SteamResult, error) {
	return SteamResult{}, fmt.Errorf("macOS 当前仅支持启动 Steam 原生游戏，不支持导入非 Steam 快捷方式")
}

func importSteamPlatformShortcuts(_ context.Context, _ []models.Game) (SteamBatchResult, error) {
	return SteamBatchResult{}, fmt.Errorf("macOS 当前仅支持启动 Steam 原生游戏，不支持导入非 Steam 快捷方式")
}

func setSteamPlatformLaunchOptions(_ context.Context, _ models.Game) (SteamResult, error) {
	return SteamResult{}, fmt.Errorf("macOS 当前仅支持启动 Steam 原生游戏，不支持写入 Steam 启动参数")
}

func nativeSteamLaunchID(game models.Game) (string, bool) {
	if !strings.EqualFold(strings.TrimSpace(game.SteamLaunchKind), "native") {
		return "", false
	}
	launchID := strings.TrimSpace(game.SteamLaunchID)
	appID, err := strconv.ParseUint(launchID, 10, 32)
	if err != nil || appID == 0 {
		return "", false
	}
	return launchID, true
}

func findDarwinSteamExecutable() (string, error) {
	homeDir, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(string(filepath.Separator), "Applications", "Steam.app", "Contents", "MacOS", "steam_osx"),
	}
	if strings.TrimSpace(homeDir) != "" {
		candidates = append(candidates,
			filepath.Join(homeDir, "Applications", "Steam.app", "Contents", "MacOS", "steam_osx"),
			filepath.Join(homeDir, "Library", "Application Support", "Steam", "Steam.AppBundle", "Steam", "Contents", "MacOS", "steam_osx"),
		)
	}

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("未找到 Steam 客户端可执行文件")
}
