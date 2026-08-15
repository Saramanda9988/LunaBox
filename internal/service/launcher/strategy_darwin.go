//go:build darwin

package launcher

import (
	"context"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	wineRunnerSystem    = "system"
	wineRunnerCrossover = "crossover"
)

type nativeAppStrategy struct{}
type nativeExecutableStrategy struct{}
type steamDarwinStrategy struct {
	steamExecutable string
}
type wineSystemStrategy struct {
	cfg *appconf.AppConfig
}
type wineCrossoverStrategy struct {
	cfg *appconf.AppConfig
}

func supportsPlatformSteamLaunch(game *models.Game) bool {
	if game == nil || !strings.EqualFold(strings.TrimSpace(game.SteamLaunchKind), "native") {
		return false
	}
	appID, err := strconv.ParseUint(strings.TrimSpace(game.SteamLaunchID), 10, 32)
	return err == nil && appID > 0
}

func selectPlatformLauncherStrategy(game *models.Game, opts LaunchOptions, cfg *appconf.AppConfig) (LauncherStrategy, error) {
	if ShouldUseSteamLaunch(game, opts) {
		if !supportsPlatformSteamLaunch(game) {
			return nil, newStrategyError("unsupported", "steam_launch_kind", "macOS 当前仅支持启动 Steam 原生游戏", fmt.Sprintf("kind=%s app_id=%s", game.SteamLaunchKind, game.SteamLaunchID))
		}
		return steamDarwinStrategy{}, nil
	}

	path := strings.TrimSpace(game.Path)
	ext := strings.ToLower(filepath.Ext(path))
	useCompatibility := enums.NormalizeLaunchMode(game.LaunchMode) == enums.LaunchModeCompatibility
	if opts.UseCompatibility != nil {
		useCompatibility = *opts.UseCompatibility
	} else if opts.WineRunner != nil {
		useCompatibility = strings.TrimSpace(*opts.WineRunner) != ""
	}

	if useCompatibility {
		if ext != ".exe" && ext != ".bat" {
			return nil, newStrategyError("invalid-config", "launch_mode", "兼容层启动仅支持 Windows 可执行文件", fmt.Sprintf("path=%s", path))
		}
		wineRunner := EffectiveString(opts.WineRunner, game.WineRunner)
		switch wineRunner {
		case "":
			return nil, newStrategyError("missing-config", "wine_runner", "请选择 Wine 或 CrossOver 启动器", fmt.Sprintf("path=%s", path))
		case wineRunnerCrossover:
			return wineCrossoverStrategy{cfg: cfg}, nil
		case wineRunnerSystem:
			return wineSystemStrategy{cfg: cfg}, nil
		default:
			return nil, newStrategyError("invalid-config", "wine_runner", "未知的兼容层启动器类型", fmt.Sprintf("wine_runner=%s", wineRunner))
		}
	}

	if ext == ".app" {
		return nativeAppStrategy{}, nil
	}

	if ext == ".exe" || ext == ".bat" {
		return nil, newStrategyError("invalid-config", "launch_mode", "Windows 可执行文件需要使用兼容层启动", fmt.Sprintf("path=%s", path))
	}
	return nativeExecutableStrategy{}, nil
}

func (s steamDarwinStrategy) Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error) {
	steamExecutable := strings.TrimSpace(s.steamExecutable)
	if steamExecutable == "" {
		var err error
		steamExecutable, err = findSteamExecutable()
		if err != nil {
			return LaunchPlan{}, err
		}
	}

	launchDirectory := strings.TrimSpace(game.GameDirectory)
	if launchDirectory == "" {
		launchDirectory = strings.TrimSpace(game.Path)
	}
	if info, err := os.Stat(launchDirectory); err != nil || !info.IsDir() {
		launchDirectory = filepath.Dir(launchDirectory)
	}
	if launchDirectory == "" || launchDirectory == "." {
		return LaunchPlan{}, fmt.Errorf("Steam 启动需要游戏安装目录用于进程检测")
	}

	return LaunchPlan{
		File:          steamExecutable,
		Args:          []string{"-silent", "steam://rungameid/" + strings.TrimSpace(game.SteamLaunchID)},
		Dir:           filepath.Dir(steamExecutable),
		DetectionDir:  launchDirectory,
		DetectionMode: DetectionSteamDirectory,
		DisplayName:   "steam_osx",
		ActiveTrack: ActiveTrack{
			Kind: ActiveTrackProcessTree,
		},
	}, nil
}

func findSteamExecutable() (string, error) {
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

func (s nativeAppStrategy) Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error) {
	return LaunchPlan{
		File:          game.Path,
		Dir:           filepath.Dir(game.Path),
		DetectionDir:  game.Path,
		DetectionMode: DetectionLauncherOnly,
		DisplayName:   filepath.Base(game.Path),
		ActiveTrack: ActiveTrack{
			Kind:       ActiveTrackBundlePath,
			BundlePath: game.Path,
		},
	}, nil
}

func (s nativeExecutableStrategy) Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error) {
	return LaunchPlan{
		File:          game.Path,
		Dir:           filepath.Dir(game.Path),
		DetectionDir:  filepath.Dir(game.Path),
		DetectionMode: DetectionLauncherOnly,
		DisplayName:   filepath.Base(game.Path),
		ActiveTrack: ActiveTrack{
			Kind: ActiveTrackLauncherPID,
		},
	}, nil
}

func (s wineSystemStrategy) Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error) {
	winePath := ""
	if s.cfg != nil {
		winePath = strings.TrimSpace(s.cfg.WineRunnerPath)
	}
	winePath, err := resolveCompatibilityBinaryPath(winePath, "wine_runner_path", "Wine")
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
	return LaunchPlan{
		File:          winePath,
		Args:          args,
		Dir:           filepath.Dir(game.Path),
		DetectionDir:  filepath.Dir(game.Path),
		Env:           env,
		DetectionMode: DetectionLauncherOnly,
		DisplayName:   filepath.Base(game.Path),
		ActiveTrack: ActiveTrack{
			Kind:           ActiveTrackWineRootPID,
			ExecutablePath: game.Path,
		},
	}, nil
}

func (s wineCrossoverStrategy) Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error) {
	winePath := ""
	if s.cfg != nil {
		winePath = strings.TrimSpace(s.cfg.CrossOverRunnerPath)
	}
	if strings.EqualFold(filepath.Ext(winePath), ".app") {
		return LaunchPlan{}, newStrategyError("invalid-config", "crossover_runner_path", "CrossOver 启动器路径应选择 bundle 内的 bin/wine，而不是 .app 本身", fmt.Sprintf("path=%s", winePath))
	}
	winePath, err := resolveCompatibilityBinaryPath(winePath, "crossover_runner_path", "CrossOver")
	if err != nil {
		return LaunchPlan{}, err
	}

	bottle := EffectiveString(opts.WinePrefix, game.WinePrefix)
	if bottle == "" && s.cfg != nil {
		bottle = strings.TrimSpace(s.cfg.CrossOverBottle)
	}

	env := []string{"WINEDEBUG=-all"}
	if bottle != "" {
		env = append(env, "CX_BOTTLE="+bottle)
	}

	wineEnv, wineArgs := parseWineCommandOptions(EffectiveString(opts.WineArgs, game.WineArgs))
	env = append(env, wineEnv...)
	args := append([]string{game.Path}, wineArgs...)
	return LaunchPlan{
		File:          winePath,
		Args:          args,
		Dir:           filepath.Dir(game.Path),
		DetectionDir:  filepath.Dir(game.Path),
		Env:           env,
		DetectionMode: DetectionLauncherOnly,
		DisplayName:   filepath.Base(game.Path),
		ActiveTrack: ActiveTrack{
			Kind:           ActiveTrackWineRootPID,
			ExecutablePath: game.Path,
			Bottle:         bottle,
		},
	}, nil
}

func resolveCompatibilityBinaryPath(path string, configKey string, runnerName string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", newStrategyError("missing-config", configKey, fmt.Sprintf("请先在设置中配置 %s 可执行文件路径", runnerName), configKey+" is empty")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", newStrategyError("missing-config", configKey, fmt.Sprintf("%s 可执行文件路径不存在：%s", runnerName, path), err.Error())
	}
	if info.IsDir() {
		return "", newStrategyError("invalid-config", configKey, fmt.Sprintf("%s 路径必须是可执行文件而不是目录：%s", runnerName, path), "compatibility runner path is a directory")
	}
	return path, nil
}
