//go:build windows

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	appName    = "LunaBox"
	launchArg  = "--autostart"
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
)

func ExtractLaunchFlag(args []string) ([]string, bool) {
	if len(args) == 0 {
		return []string{}, false
	}

	cleanArgs := make([]string, 0, len(args))
	launchedByAutostart := false

	for _, arg := range args {
		if strings.EqualFold(strings.TrimSpace(arg), launchArg) {
			launchedByAutostart = true
			continue
		}
		cleanArgs = append(cleanArgs, arg)
	}

	return cleanArgs, launchedByAutostart
}

func Sync(enabled bool) error {
	if enabled {
		return enable()
	}
	return disable()
}

func enable() error {
	command, err := buildCommand()
	if err != nil {
		return err
	}

	key, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		runKeyPath,
		registry.QUERY_VALUE|registry.SET_VALUE,
	)
	if err != nil {
		return fmt.Errorf("create startup registry key: %w", err)
	}
	defer key.Close()

	if err := key.SetStringValue(appName, command); err != nil {
		return fmt.Errorf("set startup registry value: %w", err)
	}

	return nil
}

func disable() error {
	key, err := registry.OpenKey(
		registry.CURRENT_USER,
		runKeyPath,
		registry.SET_VALUE,
	)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil
		}
		return fmt.Errorf("open startup registry key: %w", err)
	}
	defer key.Close()

	if err := key.DeleteValue(appName); err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("delete startup registry value: %w", err)
	}

	return nil
}

func buildCommand() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("get executable path: %w", err)
	}

	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return "", fmt.Errorf("normalize executable path: %w", err)
	}

	return fmt.Sprintf(`"%s" "%s"`, exePath, launchArg), nil
}
