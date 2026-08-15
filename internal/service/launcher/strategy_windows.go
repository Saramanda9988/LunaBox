//go:build windows

package launcher

import (
	"context"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

type nativeWindowsStrategy struct {
	cfg *appconf.AppConfig
}

type localeEmulatorStrategy struct {
	cfg *appconf.AppConfig
}

type steamWindowsStrategy struct{}

func supportsPlatformSteamLaunch(_ *models.Game) bool {
	return true
}

func selectPlatformLauncherStrategy(game *models.Game, opts LaunchOptions, cfg *appconf.AppConfig) (LauncherStrategy, error) {
	if ShouldUseSteamLaunch(game, opts) {
		return steamWindowsStrategy{}, nil
	}
	useLE := EffectiveBool(opts.UseLocaleEmulator, game.UseLocaleEmulator)
	if useLE && cfg != nil && cfg.LocaleEmulatorPath != "" {
		return localeEmulatorStrategy{cfg: cfg}, nil
	}
	return nativeWindowsStrategy{cfg: cfg}, nil
}

func (s nativeWindowsStrategy) Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error) {
	useMagpie := EffectiveBool(opts.UseMagpie, game.UseMagpie)
	runAsAdmin := EffectiveBool(opts.RunAsAdmin, false)
	path := game.Path
	launchDir := filepath.Dir(path)
	plan := buildStagedWindowsPlan(path, nil, launchDir, filepath.Base(path), useMagpie, runAsAdmin)
	plan.DetectionDir = EffectiveProcessDetectionDir(game.GameDirectory, launchDir)
	return plan, nil
}

func (s localeEmulatorStrategy) Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error) {
	useMagpie := EffectiveBool(opts.UseMagpie, game.UseMagpie)
	runAsAdmin := EffectiveBool(opts.RunAsAdmin, false)
	path := game.Path
	launchDir := filepath.Dir(path)
	plan := buildStagedWindowsPlan(s.cfg.LocaleEmulatorPath, []string{path}, launchDir, filepath.Base(s.cfg.LocaleEmulatorPath), useMagpie, runAsAdmin)
	plan.DetectionDir = EffectiveProcessDetectionDir(game.GameDirectory, launchDir)
	return plan, nil
}

func (s steamWindowsStrategy) Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error) {
	launchID := strings.TrimSpace(game.SteamLaunchID)
	if launchID == "" {
		return LaunchPlan{}, fmt.Errorf("此游戏尚未关联 Steam")
	}

	steamPath, err := findSteamInstallPath()
	if err != nil {
		return LaunchPlan{}, err
	}
	steamExe := filepath.Join(steamPath, "steam.exe")
	if info, err := os.Stat(steamExe); err != nil || info.IsDir() {
		return LaunchPlan{}, fmt.Errorf("未找到 steam.exe: %s", steamExe)
	}

	launchDirectory := strings.TrimSpace(game.Path)
	if info, statErr := os.Stat(launchDirectory); statErr != nil || !info.IsDir() {
		launchDirectory = filepath.Dir(launchDirectory)
	}
	if launchDirectory == "" || launchDirectory == "." {
		return LaunchPlan{}, fmt.Errorf("Steam 启动需要游戏安装目录用于进程检测")
	}
	detectionDir := EffectiveProcessDetectionDir(game.GameDirectory, launchDirectory)

	return LaunchPlan{
		File:          steamExe,
		Args:          []string{"-silent", "steam://rungameid/" + launchID},
		Dir:           steamPath,
		DetectionDir:  detectionDir,
		DetectionMode: DetectionSteamDirectory,
		DisplayName:   "steam.exe",
		Magpie:        EffectiveBool(opts.UseMagpie, game.UseMagpie),
		RunAsAdmin:    false,
		ActiveTrack: ActiveTrack{
			Kind: ActiveTrackDefault,
		},
	}, nil
}

func findSteamInstallPath() (string, error) {
	candidates := make([]string, 0, 4)
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.QUERY_VALUE)
	if err == nil {
		for _, valueName := range []string{"SteamPath", "InstallPath"} {
			value, _, valueErr := key.GetStringValue(valueName)
			if valueErr == nil && strings.TrimSpace(value) != "" {
				candidates = append(candidates, value)
			}
		}
		key.Close()
	}
	if programFilesX86 := strings.TrimSpace(os.Getenv("ProgramFiles(x86)")); programFilesX86 != "" {
		candidates = append(candidates, filepath.Join(programFilesX86, "Steam"))
	}
	if programFiles := strings.TrimSpace(os.Getenv("ProgramFiles")); programFiles != "" {
		candidates = append(candidates, filepath.Join(programFiles, "Steam"))
	}

	for _, candidate := range candidates {
		candidate = strings.ReplaceAll(strings.TrimSpace(candidate), "/", string(os.PathSeparator))
		path, pathErr := filepath.Abs(filepath.Clean(candidate))
		if pathErr != nil {
			continue
		}
		if info, err := os.Stat(filepath.Join(path, "steam.exe")); err == nil && !info.IsDir() {
			return path, nil
		}
	}

	return "", fmt.Errorf("未找到有效的 Steam 安装目录")
}
