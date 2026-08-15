//go:build !windows && !darwin && !linux

package integrator

import (
	"context"
	"fmt"
	"lunabox/internal/models"
)

func resolveSteamPlatformTarget(_ context.Context, _ models.Game) (SteamResult, error) {
	return SteamResult{
		Status: SteamLaunchStatus{
			State: SteamLaunchStateSteamNotInstalled,
		},
	}, nil
}

func importSteamPlatformShortcut(_ context.Context, _ models.Game) (SteamResult, error) {
	return SteamResult{}, fmt.Errorf("Steam integration is only supported on Windows/macOS/Linux")
}

func importSteamPlatformShortcuts(_ context.Context, _ []models.Game) (SteamBatchResult, error) {
	return SteamBatchResult{}, fmt.Errorf("Steam integration is only supported on Windows/macOS/Linux")
}

func setSteamPlatformLaunchOptions(_ context.Context, _ models.Game) (SteamResult, error) {
	return SteamResult{}, fmt.Errorf("Steam launch options are only supported on Windows/macOS/Linux")
}
