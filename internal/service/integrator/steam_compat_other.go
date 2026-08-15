//go:build !linux

package integrator

import (
	"context"
	"fmt"
	"lunabox/internal/models"
)

func getSteamPlatformCompatibilityInfo(_ context.Context, _ models.Game) (SteamCompatibilityInfo, error) {
	return SteamCompatibilityInfo{Supported: false}, nil
}

func setSteamPlatformCompatibilityTool(_ context.Context, _ models.Game, _ string) (SteamCompatibilityInfo, error) {
	return SteamCompatibilityInfo{}, fmt.Errorf("Steam Proton configuration is only supported on Linux")
}

func restartSteamPlatformClient(_ context.Context) error {
	return fmt.Errorf("Steam restart is only supported on Linux")
}
