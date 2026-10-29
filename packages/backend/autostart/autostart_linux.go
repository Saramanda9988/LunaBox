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
	path := filepath.Join(dir, autostartFileName)
	if !enabled {
		// lunabox.desktop 是本程序固定的文件名，不论 Exec 指向哪里都删：程序挪过位置（便携版）或另一个
		// 版本装在别处时，它指向的已不是当前程序，留着它下次登录会去启动一个不存在或不该启动的程序。
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("autostart: remove %s: %w", path, err)
		}
		return removeOwn(dir, exe, path)
	}
	// 换行会让 Exec 之后的内容变成新的键（同 Wails 的 validateDesktopExecToken）。
	if strings.ContainsFunc(exe, func(r rune) bool { return r != '\t' && (r < 0x20 || r == 0x7f) }) {
		return fmt.Errorf("autostart: control character in executable path %q", exe)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("autostart: create autostart dir: %w", err)
	}
	// 先写临时文件再改名：写到一半失败不会留下一个残缺的启动项。
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(desktopEntry(exe)), 0o644); err != nil {
		return fmt.Errorf("autostart: write desktop file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("autostart: write desktop file: %w", err)
	}
	// 别的文件名下指向本程序的旧项（如 Wails 版按别的标识写的）一并删掉，免得登录时启动两次。
	return removeOwn(dir, exe, path)
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

// removeOwn 删除 keep 以外 Exec 指向 exe 的 .desktop：只删本程序的启动项，不动其他程序的。
func removeOwn(dir, exe, keep string) error {
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
		if path == keep {
			continue
		}
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

// quoteExec 按 freedesktop 的 Exec 规则给一个参数加引号：含空格或保留字符时双引号并转义，
// % 写成 %%（否则 %f、%u 之类会被启动器当作字段码展开）。引号内的转义与 Wails 一样只转一层，
// 两个版本才认得出彼此写的 Exec。
func quoteExec(s string) string {
	s = strings.ReplaceAll(s, "%", "%%")
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

// execPathOf 返回 Exec= 一行的第一个参数（去掉引号与转义，%% 还原为 %）。
func execPathOf(contents string) string {
	return strings.ReplaceAll(execToken(contents), "%%", "%")
}

func execToken(contents string) string {
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
