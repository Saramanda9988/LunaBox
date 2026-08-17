//go:build windows

package updateclient

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func configureUpdateHelperCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
}
