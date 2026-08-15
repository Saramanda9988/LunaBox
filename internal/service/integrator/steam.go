package integrator

import (
	"context"
	"lunabox/internal/models"
)

const (
	SteamLaunchStateReady              = "ready"
	SteamLaunchStateNeedsImport        = "needs_import"
	SteamLaunchStateSteamNotInstalled  = "steam_not_installed"
	SteamLaunchStateSteamRunning       = "steam_running"
	SteamLaunchStateExecutableRequired = "executable_required"
	SteamLaunchStateUserUnavailable    = "user_unavailable"
)

type SteamLaunchStatus struct {
	State          string
	Ready          bool
	SteamInstalled bool
	SteamRunning   bool
	LaunchID       string
	LaunchKind     string
	UserID         string
	ProtonPrefix   string
}

type SteamResult struct {
	Status     SteamLaunchStatus
	Imported   bool
	BackupPath string
}

type SteamBatchItemResult struct {
	GameID string
	Result SteamResult
	Err    error
}

type SteamBatchResult struct {
	Items      []SteamBatchItemResult
	BackupPath string
}

type SteamCompatibilityTool struct {
	Name        string
	DisplayName string
	Path        string
	BuiltIn     bool
}

type SteamCompatibilityInfo struct {
	Supported      bool
	SteamInstalled bool
	SteamRoot      string
	AppID          string
	ProtonPrefix   string
	CurrentTool    string
	DefaultTool    string
	Tools          []SteamCompatibilityTool
}

func ResolveSteamTarget(ctx context.Context, game models.Game) (SteamResult, error) {
	return resolveSteamPlatformTarget(ctx, game)
}

func ImportSteamShortcut(ctx context.Context, game models.Game) (SteamResult, error) {
	return importSteamPlatformShortcut(ctx, game)
}

func ImportSteamShortcuts(ctx context.Context, games []models.Game) (SteamBatchResult, error) {
	return importSteamPlatformShortcuts(ctx, games)
}

func SetSteamLaunchOptions(ctx context.Context, game models.Game, launchOptions string) (SteamResult, error) {
	game.SteamLaunchOptions = launchOptions
	return setSteamPlatformLaunchOptions(ctx, game)
}

func GetSteamCompatibilityInfo(ctx context.Context, game models.Game) (SteamCompatibilityInfo, error) {
	return getSteamPlatformCompatibilityInfo(ctx, game)
}

func SetSteamCompatibilityTool(ctx context.Context, game models.Game, toolName string) (SteamCompatibilityInfo, error) {
	return setSteamPlatformCompatibilityTool(ctx, game, toolName)
}

func RestartSteamClient(ctx context.Context) error {
	return restartSteamPlatformClient(ctx)
}
