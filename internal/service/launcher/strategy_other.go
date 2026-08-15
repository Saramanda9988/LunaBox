//go:build !windows && !darwin && !linux

package launcher

import (
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
)

func supportsPlatformSteamLaunch(_ *models.Game) bool {
	return false
}

func selectPlatformLauncherStrategy(game *models.Game, opts LaunchOptions, cfg *appconf.AppConfig) (LauncherStrategy, error) {
	return nil, fmt.Errorf("launcher strategies are only supported on Windows, macOS and Linux")
}
