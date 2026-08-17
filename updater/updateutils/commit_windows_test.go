//go:build windows

package updateutils

import (
	"os/exec"
	"testing"

	"golang.org/x/sys/windows"
)

func TestConfigureRestartCommandHidesConsole(t *testing.T) {
	command := exec.Command("LunaBox.exe")
	if err := configureRestartCommand(command); err != nil {
		t.Fatal(err)
	}

	if command.SysProcAttr == nil {
		t.Fatal("expected Windows process attributes")
	}
	if !command.SysProcAttr.HideWindow {
		t.Fatal("expected restart window to be hidden")
	}
	if command.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatal("expected restart to use CREATE_NO_WINDOW")
	}
	if command.SysProcAttr.CreationFlags&windows.CREATE_NEW_PROCESS_GROUP == 0 {
		t.Fatal("expected restart to use a new process group")
	}
}
