//go:build windows

package plugin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/windows"
)

const createNoWindow = 0x08000000

// shellCommand 交给 cmd.exe 执行整行命令。直接写 CmdLine，避免 Go 按 C 运行时规则转义引号（cmd.exe 不认 \"）。
func shellCommand(ctx context.Context, line string) *exec.Cmd {
	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		comspec = filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
	}
	cmd := exec.CommandContext(ctx, comspec)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `"` + comspec + `" /d /s /c "` + line + `"`}
	return cmd
}

// configureProcessAttrs 不弹出控制台窗口。
func configureProcessAttrs(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}

// killProcessTree 结束进程及其子进程（cmd.exe 启动的命令会留下子进程）。
func killProcessTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
	configureProcessAttrs(kill)
	if err := kill.Run(); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}

// systemLegacyEncoding 按系统 ANSI 代码页给出控制台输出的旧编码。
func systemLegacyEncoding() string {
	switch windows.GetACP() {
	case 936:
		return "gbk"
	case 950:
		return "big5"
	case 932:
		return "shift_jis"
	case 949:
		return "euc-kr"
	}
	return "utf8"
}
