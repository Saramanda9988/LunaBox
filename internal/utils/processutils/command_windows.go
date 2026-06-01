//go:build windows

package processutils

import (
	"os/exec"
	"syscall"
)

const (
	detachedProcessFlag uint32 = 0x00000008
	createNoWindowFlag  uint32 = 0x08000000
)

// ConfigureDetachedCommand 防止外部程序附着到 LunaBox 的开发控制台。
func ConfigureDetachedCommand(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= detachedProcessFlag | createNoWindowFlag
}
