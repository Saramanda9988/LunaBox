//go:build linux

package focusing

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// 每次查询都要起外部进程，结果缓存一段时间；缓存期内返回旧值，只会让暂停/恢复晚几秒，不会少记时长。
	linuxIdleCacheTTL = 5 * time.Second
	// 所有来源都不可用时，隔久一点再重新探测。
	linuxIdleUnsupportedRetry = time.Minute
	linuxIdleQueryTimeout     = 800 * time.Millisecond
)

// linuxIdleSource 是一种查询空闲时长（毫秒）的方式。
type linuxIdleSource struct {
	name    string
	usable  func() bool
	command []string
}

// Linux 没有跨桌面的空闲 API：
//   - GNOME（X11/Wayland）走 Mutter IdleMonitor，单位毫秒；
//   - 其它 X11 桌面走 xprintidle（XScreenSaver 扩展），单位毫秒；
//   - KDE Wayland 的 org.freedesktop.ScreenSaver.GetSessionIdleTime 直接返回 NotSupported，暂无可用来源。
var linuxIdleSources = []linuxIdleSource{
	{
		name:   "mutter",
		usable: func() bool { return true },
		command: []string{
			"gdbus", "call", "--session",
			"--dest", "org.gnome.Mutter.IdleMonitor",
			"--object-path", "/org/gnome/Mutter/IdleMonitor/Core",
			"--method", "org.gnome.Mutter.IdleMonitor.GetIdletime",
		},
	},
	{
		name:    "xprintidle",
		usable:  isX11Session,
		command: []string{"xprintidle"},
	},
}

var runLinuxIdleCommand = func(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), linuxIdleQueryTimeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Output()
}

var linuxIdle linuxIdleState

type linuxIdleState struct {
	mu        sync.Mutex
	source    int // 上次成功的来源下标 +1，0 表示未知
	checkedAt time.Time
	idle      time.Duration
	ok        bool
}

// GetIdleDuration 返回用户最近一次输入距今的时长；当前桌面不支持时返回 false。
func GetIdleDuration() (time.Duration, bool) {
	return linuxIdle.get(time.Now())
}

func (s *linuxIdleState) get(now time.Time) (time.Duration, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.checkedAt.IsZero() {
		ttl := linuxIdleCacheTTL
		if !s.ok {
			ttl = linuxIdleUnsupportedRetry
		}
		if now.Sub(s.checkedAt) < ttl {
			return s.idle, s.ok
		}
	}

	s.checkedAt = now
	s.idle, s.ok = 0, false

	// 先试上次成功的来源，避免每次都先跑一遍注定失败的命令。
	if s.source > 0 {
		if idle, ok := queryLinuxIdleSource(linuxIdleSources[s.source-1]); ok {
			s.idle, s.ok = idle, true
			return s.idle, s.ok
		}
	}
	for i, source := range linuxIdleSources {
		if i+1 == s.source {
			continue
		}
		if idle, ok := queryLinuxIdleSource(source); ok {
			s.source = i + 1
			s.idle, s.ok = idle, true
			return s.idle, s.ok
		}
	}
	s.source = 0
	return 0, false
}

func queryLinuxIdleSource(source linuxIdleSource) (time.Duration, bool) {
	if !source.usable() {
		return 0, false
	}
	out, err := runLinuxIdleCommand(source.command[0], source.command[1:]...)
	if err != nil {
		return 0, false
	}
	millis, ok := parseIdleMillis(string(out))
	if !ok {
		return 0, false
	}
	return time.Duration(millis) * time.Millisecond, true
}

// parseIdleMillis 解析 xprintidle 的 "1234" 或 gdbus 的 "(uint64 1234,)"。
func parseIdleMillis(output string) (uint64, bool) {
	for _, field := range strings.Fields(output) {
		value, err := strconv.ParseUint(strings.Trim(field, "(),"), 10, 64)
		if err == nil {
			return value, true
		}
	}
	return 0, false
}

func isX11Session() bool {
	if os.Getenv("DISPLAY") == "" {
		return false
	}
	// XWayland 下 XScreenSaver 只能看到 X 客户端的输入，结果不可靠。
	return os.Getenv("WAYLAND_DISPLAY") == "" && !strings.EqualFold(os.Getenv("XDG_SESSION_TYPE"), "wayland")
}
