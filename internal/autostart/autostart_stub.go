//go:build !windows

package autostart

import "fmt"

func ExtractLaunchFlag(args []string) ([]string, bool) {
	return args, false
}

func Sync(enabled bool) error {
	if !enabled {
		return nil
	}
	return fmt.Errorf("autostart is only supported on Windows")
}
