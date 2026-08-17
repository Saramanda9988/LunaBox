//go:build windows

package updateclient

import (
	"os/exec"
	"testing"

	"golang.org/x/sys/windows"
)

func TestConfigureUpdateHelperCommandHidesConsole(t *testing.T) {
	command := exec.Command("LunaBoxUpdater.exe")
	configureUpdateHelperCommand(command)

	if command.SysProcAttr == nil {
		t.Fatal("expected Windows process attributes")
	}
	if !command.SysProcAttr.HideWindow {
		t.Fatal("expected updater window to be hidden")
	}
	if command.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatal("expected updater to use CREATE_NO_WINDOW")
	}
}
