//go:build !windows && !linux

package autostart

import "lunabox/backend/appruntime"

// Set 只在 Windows 与 Linux 上实现；其他系统上由 Wails 的 Autostart 负责。
func Set(bool) error { return appruntime.ErrUnavailable }
