package launcher

import (
	"context"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"lunabox/internal/utils/timerutils"
	"os"
	"path/filepath"
	"strings"
)

type DetectionMode int

const (
	DetectionStaged DetectionMode = iota
	DetectionLauncherOnly
	DetectionSteamDirectory
)

type ExitWatchMode int

const (
	ExitWatchDisabled ExitWatchMode = iota
	ExitWatchGameProcessPresence
)

type ActiveTrack = timerutils.ActiveTrack

const (
	ActiveTrackDefault     = timerutils.ActiveTrackDefault
	ActiveTrackBundlePath  = timerutils.ActiveTrackBundlePath
	ActiveTrackProcessTree = timerutils.ActiveTrackProcessTree
	ActiveTrackWineRootPID = timerutils.ActiveTrackWineRootPID
	ActiveTrackLauncherPID = timerutils.ActiveTrackLauncherPID
)

type ExitWatch struct {
	Mode              ExitWatchMode
	DetectionDir      string
	IgnoreRootProcess bool
}

type LaunchPlan struct {
	File          string
	Args          []string
	Dir           string
	Env           []string
	DetectionDir  string
	DetectionMode DetectionMode
	DisplayName   string
	ActiveTrack   ActiveTrack
	ExitWatch     ExitWatch
	Magpie        bool
	RunAsAdmin    bool
}

// LaunchOptions defines optional game launch overrides.
type LaunchOptions struct {
	UseLocaleEmulator *bool
	UseMagpie         *bool
	RunAsAdmin        *bool
	WineRunner        *string
	WineArgs          *string
	WinePrefix        *string
	UseSteam          *bool
	UseCompatibility  *bool
}

type LauncherStrategy interface {
	Plan(ctx context.Context, game *models.Game, opts LaunchOptions) (LaunchPlan, error)
}

type StrategyError struct {
	Kind        string
	ConfigKey   string
	UserMessage string
	Detail      string
}

func (e *StrategyError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Detail) != "" {
		return fmt.Sprintf("%s: %s", e.UserMessage, e.Detail)
	}
	return e.UserMessage
}

func newStrategyError(kind string, configKey string, userMessage string, detail string) *StrategyError {
	return &StrategyError{
		Kind:        strings.TrimSpace(kind),
		ConfigKey:   strings.TrimSpace(configKey),
		UserMessage: strings.TrimSpace(userMessage),
		Detail:      strings.TrimSpace(detail),
	}
}

func SelectLauncherStrategy(game *models.Game, opts LaunchOptions, cfg *appconf.AppConfig) (LauncherStrategy, error) {
	if game == nil {
		return nil, fmt.Errorf("game is nil")
	}
	return selectPlatformLauncherStrategy(game, opts, cfg)
}

func ShouldUseSteamLaunch(game *models.Game, opts LaunchOptions) bool {
	if game == nil {
		return false
	}
	if opts.UseSteam != nil {
		return *opts.UseSteam
	}
	return enums.NormalizeLaunchMode(game.LaunchMode) == enums.LaunchModeSteam
}

func SupportsSteamLaunch(game *models.Game, opts LaunchOptions) bool {
	return ShouldUseSteamLaunch(game, opts) && supportsPlatformSteamLaunch(game)
}

func EffectiveBool(option *bool, fallback bool) bool {
	if option != nil {
		return *option
	}
	return fallback
}

func EffectiveString(option *string, fallback string) string {
	if option != nil {
		return strings.TrimSpace(*option)
	}
	return strings.TrimSpace(fallback)
}

func parseWineArgs(args string) []string {
	args = strings.TrimSpace(args)
	if args == "" {
		return nil
	}
	return strings.Fields(args)
}

func parseWineCommandOptions(args string) ([]string, []string) {
	fields := parseWineArgs(args)
	if len(fields) == 0 {
		return nil, nil
	}

	env := make([]string, 0)
	commandArgs := make([]string, 0, len(fields))
	allowEnvPrefix := true
	for _, field := range fields {
		if field == "%command%" {
			allowEnvPrefix = false
			continue
		}
		if allowEnvPrefix && isWineEnvAssignment(field) {
			env = append(env, field)
			continue
		}
		allowEnvPrefix = false
		commandArgs = append(commandArgs, field)
	}
	return env, commandArgs
}

func isWineEnvAssignment(value string) bool {
	equalsIndex := strings.IndexByte(value, '=')
	if equalsIndex <= 0 {
		return false
	}
	name := value[:equalsIndex]
	for index, char := range name {
		if char == '_' || ('A' <= char && char <= 'Z') || ('a' <= char && char <= 'z') {
			continue
		}
		if index > 0 && '0' <= char && char <= '9' {
			continue
		}
		return false
	}
	return true
}

// EffectiveProcessDetectionDir returns the configured game root when it is a
// valid parent of the launch directory. The launch directory remains the safe
// fallback for stale or unrelated game_directory values.
func EffectiveProcessDetectionDir(gameDirectory string, launchDirectory string) string {
	fallback := strings.TrimSpace(launchDirectory)
	configured := strings.TrimSpace(gameDirectory)
	if configured == "" {
		return fallback
	}

	configuredAbs, err := filepath.Abs(filepath.Clean(configured))
	if err != nil {
		return fallback
	}
	info, err := os.Stat(configuredAbs)
	if err != nil || !info.IsDir() {
		return fallback
	}

	if fallback == "" {
		return configuredAbs
	}
	fallbackAbs, err := filepath.Abs(filepath.Clean(fallback))
	if err != nil {
		return fallback
	}
	relative, err := filepath.Rel(configuredAbs, fallbackAbs)
	if err != nil || filepath.IsAbs(relative) || relative == ".." ||
		strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return fallback
	}

	return configuredAbs
}

func buildStagedWindowsPlan(file string, args []string, dir string, displayName string, useMagpie bool, runAsAdmin bool) LaunchPlan {
	if strings.TrimSpace(displayName) == "" {
		displayName = filepath.Base(file)
	}
	return LaunchPlan{
		File:          file,
		Args:          args,
		Dir:           dir,
		DetectionDir:  dir,
		DetectionMode: DetectionStaged,
		DisplayName:   displayName,
		Magpie:        useMagpie,
		RunAsAdmin:    runAsAdmin,
		ActiveTrack: ActiveTrack{
			Kind: ActiveTrackDefault,
		},
	}
}
