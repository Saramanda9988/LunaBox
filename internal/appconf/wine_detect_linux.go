//go:build linux

package appconf

import (
	"lunabox/internal/applog"
	"os/exec"
	"strings"
)

func detectDefaultCrossOverRunnerPath(config *AppConfig) bool {
	return false
}

func detectDefaultWineRunnerPath(config *AppConfig) bool {
	if config == nil || strings.TrimSpace(config.WineRunnerPath) != "" {
		return false
	}

	for _, candidate := range []string{"wine", "wine64"} {
		path, err := exec.LookPath(candidate)
		if err == nil && strings.TrimSpace(path) != "" {
			config.WineRunnerPath = path
			applog.LogInfof(nil, "Detected Wine binary at %s", path)
			return true
		}
	}
	return false
}
