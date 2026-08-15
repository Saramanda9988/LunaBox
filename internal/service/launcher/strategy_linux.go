//go:build linux

package launcher

import (
	"context"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	wineRunnerSystem = "system"
	wineRunnerCustom = "custom"
)

type nativeLinuxStrategy struct{}
type wineLinuxStrategy struct {
	cfg *appconf.AppConfig
}
type steamLinuxStrategy struct{}

func supportsPlatformSteamLaunch(_ *models.Game) bool {
	return true
}

func selectPlatformLauncherStrategy(game *models.Game, opts LaunchOptions, cfg *appconf.AppConfig) (LauncherStrategy, error) {
	if ShouldUseSteamLaunch(game, opts) {
		return steamLinuxStrategy{}, nil
	}

	path := strings.TrimSpace(game.Path)
	ext := strings.ToLower(filepath.Ext(path))
	wineRunner := effectiveLinuxWineRunner(path, EffectiveString(opts.WineRunner, game.WineRunner))

	if ext == ".exe" || ext == ".bat" {
		switch wineRunner {
		case wineRunnerSystem, wineRunnerCustom:
			return wineLinuxStrategy{cfg: cfg}, nil
		case "crossover":
			return nil, newStrategyError("invalid-config", "wine_runner", "Linux 暂不支持 CrossOver 启动器，请改用系统 Wine 或自定义 Wine", fmt.Sprintf("wine_runner=%s", wineRunner))
		default:
			return nil, newStrategyError("invalid-config", "wine_runner", "未知的 Wine 启动器类型", fmt.Sprintf("wine_runner=%s", wineRunner))
		}
	}

	if wineRunner != "" {
		return nil, newStrategyError("invalid-config", "wine_runner", "原生 Linux 可执行文件不应启用 Wine 启动器", fmt.Sprintf("path=%s wine_runner=%s", path, wineRunner))
	}
	return nativeLinuxStrategy{}, nil
}

func (s nativeLinuxStrategy) Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error) {
	launchDir := filepath.Dir(game.Path)
	return LaunchPlan{
		File:          game.Path,
		Dir:           launchDir,
		DetectionDir:  EffectiveProcessDetectionDir(game.GameDirectory, launchDir),
		DetectionMode: DetectionLauncherOnly,
		DisplayName:   filepath.Base(game.Path),
		ActiveTrack: ActiveTrack{
			Kind: ActiveTrackDefault,
		},
		ExitWatch: ExitWatch{
			Mode: ExitWatchGameProcessPresence,
		},
	}, nil
}

func (s wineLinuxStrategy) Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error) {
	winePath, err := resolveLinuxWineBinaryPath(s.cfg)
	if err != nil {
		return LaunchPlan{}, err
	}

	prefix := EffectiveString(opts.WinePrefix, game.WinePrefix)
	if prefix == "" && s.cfg != nil {
		prefix = strings.TrimSpace(s.cfg.WinePrefix)
	}

	env := []string{"WINEDEBUG=-all"}
	if prefix != "" {
		env = append(env, "WINEPREFIX="+prefix)
	}
	wineEnv, wineArgs := parseWineCommandOptions(EffectiveString(opts.WineArgs, game.WineArgs))
	env = append(env, wineEnv...)
	args := append([]string{game.Path}, wineArgs...)
	launchDir := filepath.Dir(game.Path)
	return LaunchPlan{
		File:          winePath,
		Args:          args,
		Dir:           launchDir,
		DetectionDir:  EffectiveProcessDetectionDir(game.GameDirectory, launchDir),
		Env:           env,
		DetectionMode: DetectionStaged,
		DisplayName:   filepath.Base(game.Path),
		ActiveTrack: ActiveTrack{
			Kind: ActiveTrackWineRootPID,
		},
		ExitWatch: ExitWatch{
			Mode: ExitWatchGameProcessPresence,
		},
	}, nil
}

func (s steamLinuxStrategy) Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error) {
	launchID := strings.TrimSpace(game.SteamLaunchID)
	if launchID == "" {
		launchID = strings.TrimSpace(game.SourceID)
	}
	if launchID == "" {
		return LaunchPlan{}, fmt.Errorf("此游戏尚未关联 Steam")
	}

	file, args, displayName, err := resolveLinuxSteamCommand(launchID)
	if err != nil {
		return LaunchPlan{}, err
	}

	installDir := strings.TrimSpace(game.Path)
	if installDir == "" {
		return LaunchPlan{}, fmt.Errorf("Steam 启动需要游戏安装目录用于进程检测")
	}
	detectionDir := EffectiveProcessDetectionDir(game.GameDirectory, installDir)
	workingDir := detectionDir
	if info, err := os.Stat(workingDir); err != nil || !info.IsDir() {
		workingDir = ""
	}

	return LaunchPlan{
		File:          file,
		Args:          args,
		Dir:           workingDir,
		DetectionDir:  detectionDir,
		DetectionMode: DetectionSteamDirectory,
		DisplayName:   displayName,
		ActiveTrack: ActiveTrack{
			Kind: ActiveTrackDefault,
		},
		ExitWatch: ExitWatch{
			Mode: ExitWatchGameProcessPresence,
		},
	}, nil
}

func effectiveLinuxWineRunner(path string, runner string) string {
	runner = strings.TrimSpace(runner)
	if runner != "" {
		return runner
	}
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(path))) {
	case ".exe", ".bat":
		return wineRunnerSystem
	default:
		return ""
	}
}

func resolveLinuxWineBinaryPath(cfg *appconf.AppConfig) (string, error) {
	if cfg == nil || strings.TrimSpace(cfg.WineRunnerPath) == "" {
		return "", newStrategyError("missing-config", "wine_runner_path", "请先在设置中配置 Wine 可执行文件路径", "WineRunnerPath is empty")
	}

	winePath := strings.TrimSpace(cfg.WineRunnerPath)
	info, err := os.Stat(winePath)
	if err != nil {
		return "", newStrategyError("missing-config", "wine_runner_path", fmt.Sprintf("Wine 可执行文件路径不存在：%s", winePath), err.Error())
	}
	if info.IsDir() {
		return "", newStrategyError("invalid-config", "wine_runner_path", fmt.Sprintf("Wine 路径必须是可执行文件而不是目录：%s", winePath), "wine runner path is a directory")
	}
	return winePath, nil
}

func resolveLinuxSteamCommand(sourceID string) (string, []string, string, error) {
	launchURL := "steam://rungameid/" + strings.TrimSpace(sourceID)
	if steamPath, err := exec.LookPath("steam"); err == nil && strings.TrimSpace(steamPath) != "" {
		return steamPath, []string{launchURL}, filepath.Base(steamPath), nil
	}
	if flatpakPath, err := exec.LookPath("flatpak"); err == nil && strings.TrimSpace(flatpakPath) != "" {
		return flatpakPath, []string{"run", "com.valvesoftware.Steam", launchURL}, "flatpak", nil
	}
	return "", nil, "", fmt.Errorf("未找到 Steam 启动命令：请安装 steam 命令，或安装 com.valvesoftware.Steam Flatpak")
}
