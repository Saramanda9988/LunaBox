//go:build linux

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"lunabox/backend/appruntime"
	"lunabox/backend/utils/apputils"
)

// Wails 的 Autostart（application.autostartSlug("LunaBox")）在 ~/.config/autostart 下写
// lunabox.desktop，Exec 是 `"<程序>" --autostart`。这里写同一个文件，Wails 版与 MyGo 版共用
// 一个开机启动设置，两个版本互相替换而不会各留一个。
const (
	autostartFileName = "lunabox.desktop"
	autostartAppName  = "LunaBox"
)

// Set 打开或关闭当前程序的开机自启动。
func Set(enabled bool) error {
	exe, err := apputils.GetLaunchExecutablePath()
	if err != nil {
		return fmt.Errorf("autostart: get executable path: %w", err)
	}
	dir, err := autostartDir()
	if err != nil {
		return err
	}
	if !enabled {
		return removeOwn(dir, exe)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("autostart: create autostart dir: %w", err)
	}
	// 换过安装位置时先删掉指向同一个程序的旧项。
	if err := removeOwn(dir, exe); err != nil {
		return err
	}
	path := filepath.Join(dir, autostartFileName)
	if err := os.WriteFile(path, []byte(desktopEntry(exe)), 0o644); err != nil {
		return fmt.Errorf("autostart: write desktop file: %w", err)
	}
	return nil
}

// autostartDir 与 Wails 一致：$XDG_CONFIG_HOME/autostart，未设置时 ~/.config/autostart。
func autostartDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); dir != "" {
		return filepath.Join(dir, "autostart"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("autostart: %w", err)
	}
	return filepath.Join(home, ".config", "autostart"), nil
}

// desktopEntry 与 Wails 写出的内容一致。
func desktopEntry(exe string) string {
	return fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Exec=%s %s
X-GNOME-Autostart-enabled=true
Hidden=false
NoDisplay=false
Terminal=false
`, autostartAppName, quoteExec(exe), appruntime.AutostartLaunchArgument)
}

// removeOwn 删除 Exec 指向 exe 的 .desktop：只删本程序的启动项，不动其他程序的。
func removeOwn(dir, exe string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("autostart: read autostart dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".desktop") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if execPathOf(string(content)) != exe {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("autostart: remove %s: %w", path, err)
		}
	}
	return nil
}

// quoteExec 按 freedesktop 的 Exec 规则给一个参数加引号：含空格或保留字符时双引号并转义。
func quoteExec(s string) string {
	if !strings.ContainsAny(s, " \t\"`$\\") {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		if r == '"' || r == '`' || r == '$' || r == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}

// execPathOf 返回 Exec= 一行的第一个参数（去掉引号与转义）。
func execPathOf(contents string) string {
	for _, line := range strings.Split(contents, "\n") {
		value, ok := strings.CutPrefix(strings.TrimSpace(line), "Exec=")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if !strings.HasPrefix(value, `"`) {
			if i := strings.IndexAny(value, " \t"); i >= 0 {
				return value[:i]
			}
			return value
		}
		var b strings.Builder
		for i := 1; i < len(value); i++ {
			switch value[i] {
			case '\\':
				if i+1 < len(value) {
					b.WriteByte(value[i+1])
					i++
				}
			case '"':
				return b.String()
			default:
				b.WriteByte(value[i])
			}
		}
		return ""
	}
	return ""
}
